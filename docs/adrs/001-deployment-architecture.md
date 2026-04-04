# ADR-001: Deployment Architecture

**Status:** Accepted
**Date:** 2026-03-27

## Context

Vibe Seeker runs locally via `compose.yml` (Go backend + React frontend + PostgreSQL). We need to make it publicly accessible at a custom domain so it can be shared and demoed. The primary use case is a personal project shown to friends on demand.

**Constraints:**
- Cost is the #1 priority — target under $5/month
- Must be always available (no manual start/stop workflow)
- Infrastructure defined as code (Terraform)
- Deployed via GitHub Actions with no stored AWS keys
- Spotify development apps are limited to 25 users without quota extension

## Decision

### Cloud Provider: AWS

AWS over GCP because:
- All existing project planning docs target AWS
- CloudFront's free tier (1 TB transfer, 10M requests/month) eliminates CDN cost
- CloudFront natively supports path-based routing to multiple origins (S3 + App Runner) without requiring a load balancer
- GCP's equivalent requires an Application Load Balancer (~$18/month always-on), erasing Cloud Run's scale-to-zero savings

All resources in `us-east-1` (required for CloudFront ACM certificates, closest to NYC target users).

### Frontend: S3 + CloudFront

```
vibeseeker.dev
      |
 [CloudFront] + Shield Standard (free)
  /          \
/*           /api/*
 |              |
[S3]       [App Runner]
```

Single CloudFront distribution with path-based routing:
- Default behavior (`/*`): S3 origin via Origin Access Control (private bucket)
- Ordered behavior (`/api/*`): App Runner custom origin

A CloudFront Function rewrites non-file URIs to `/index.html` for SPA routing (mirrors nginx `try_files`). The `/api/*` behavior uses `AllViewerExceptHostHeader` origin request policy to forward session cookies to App Runner. The default S3 behavior uses `CachingOptimized` which strips cookies.

This single-domain setup eliminates CORS issues and keeps the Spotify OAuth redirect URI stable.

### Backend: App Runner

App Runner (0.25 vCPU, 0.5 GB) runs the Go container from ECR. Always on with min/max instances set to 1.

- Auto-deploys when a new `:latest` image is pushed to ECR
- Health check on `/api/health`
- Runtime secrets injected via SSM Parameter Store references (`runtime_environment_secrets`)
- Cost: ~$1.58/month idle

We considered a pause/resume toggle to save the $1.58/month idle cost, but the operational friction (running a workflow and waiting 30-60s before demoing) isn't worth the savings.

### Database: Neon (Serverless Postgres)

Neon over RDS because:
- **Scales to zero automatically** — compute suspends after ~5 min idle, wakes in ~500ms on connection
- **Free tier** — 0.5 GB storage, 191 compute hours/month (sufficient for a demo app)
- **No lifecycle management** — no start/stop workflow, no 7-day auto-restart problem, no Lambda watchdog
- **No VPC complexity** — RDS in a VPC would require a NAT Gateway (~$32/month) for App Runner to reach external APIs (Spotify, Last.fm, Ticketmaster). Public RDS was the alternative, but Neon's managed proxy with TLS + random endpoint IDs is more secure.
- **Portable** — standard `pg_dump` for backups, `pgx` driver works without code changes
- **Terraform-managed** — `kislerdm/neon` provider creates project, database, role, and endpoint

Neon project runs in `aws-us-east-1` to minimize latency to App Runner. The pooled connection endpoint (PgBouncer) is used by default.

RDS `db.t4g.micro` would have cost ~$14/month running + $2.30/month stopped (storage), plus required a Lambda + EventBridge auto-re-stop mechanism for the 7-day restart problem. Neon eliminates all of this at $0/month.

### Container Registry: ECR

App Runner can only pull from ECR (no native GHCR support). ECR costs ~$0.10/month for image storage with a lifecycle policy keeping the last 5 images.

### WAF: Not Initially

AWS WAF costs $8/month ($5 web ACL + $1/rule) — more than all other services combined for a demo app. Sufficient free-tier protection exists:

1. Shield Standard (free with CloudFront) — network-layer DDoS
2. App Runner max_size=1 — compute cost ceiling of ~$14/month under sustained attack
3. Authenticated endpoints — most `/api/*` routes require valid JWT, bots get 401
4. Turnstile — anonymous auth requires valid bot-detection token
5. Application-level rate limiting (`internal/ratelimit/`)
6. `robots.txt` + `noindex` meta tags

### CI/CD: GitHub Actions with OIDC

GitHub Actions authenticates to AWS via OIDC federation (no stored credentials). A `vibe-seeker-deploy` IAM role with least-privilege permissions:
- ECR: authenticate + push images
- S3: sync frontend assets
- CloudFront: create invalidations

Three workflows:
- `deploy-frontend.yml` — on push to main (`frontend/**`): build, S3 sync, CloudFront invalidation
- `deploy-backend.yml` — on push to main (`backend/**`): build container, push to ECR (App Runner auto-deploys)
- `tf-plan-apply.yml` — on PR (`infra/**`): plan + PR comment; on merge: apply

### Secrets Management: SSM Parameter Store

Runtime secrets (API keys, JWT secret, DATABASE_URL) stored in SSM Parameter Store as SecureString. App Runner resolves SSM references at instance startup.

Build-time values (deploy role ARN, bucket names, Turnstile site key) stored as GitHub repository variables.

SSM hierarchy: `/vibe-seeker/{env}/{secret-name}`

DATABASE_URL is auto-populated by Terraform from Neon module outputs. All other secrets are created with placeholder values (`lifecycle { ignore_changes = [value] }`) and populated manually via `aws ssm put-parameter`.

### Domain Registration

`.dev` TLD (requires HTTPS, which CloudFront + ACM provides). Recommended registrars:
- **Porkbun** (~$10/year) — best value, requires manual NS delegation to Route 53
- **Cloudflare** (~$10/year) — at-cost pricing, already used for Turnstile
- **Route 53** ($14/year) — most convenient, auto-configured NS

Route 53 hosted zone + ACM certificate managed by Terraform.

## Infrastructure as Code

### Terraform Structure

```
infra/
├── modules/
│   ├── cdn/          # S3 + CloudFront + SPA rewrite function
│   ├── compute/      # App Runner + IAM roles + auto-scaling
│   ├── database/     # Neon project + branch + database + role + endpoint
│   ├── dns/          # Route 53 hosted zone + ACM certificate
│   ├── ecr/          # Container registry + lifecycle policy
│   └── cicd/         # GitHub OIDC provider + deploy IAM role
├── bootstrap/        # S3 + DynamoDB for TF state (run once, local state)
└── prod/             # Production environment composition
    ├── main.tf       # Wires modules together + SSM parameters
    ├── variables.tf
    ├── outputs.tf
    ├── backend.tf    # S3 remote state config
    ├── providers.tf  # AWS + Neon provider config
    └── terraform.tfvars
```

### Environment-Specific Values

| Layer | Tool | Contents |
|-------|------|----------|
| Infrastructure config | `terraform.tfvars` per env | Instance sizes, domain name, Neon region |
| Application secrets | AWS SSM Parameter Store | API keys, JWT secret, DATABASE_URL |
| Build-time config | GitHub Actions variables | Turnstile site key, deploy role ARN, bucket names |

## Cost

### Monthly: ~$2.19

| Service | Cost | Notes |
|---------|------|-------|
| App Runner (idle) | $1.58 | 0.25 vCPU + 0.5 GB provisioned |
| Route 53 | $0.50 | Hosted zone |
| ECR | ~$0.10 | Container image storage |
| S3 + CloudFront | ~$0.01 | Free tier covers demo traffic |
| Neon | $0.00 | Free tier |
| SSM | $0.00 | Standard parameters free |

Domain registration: ~$10-14/year.

### Alternatives Considered

| Alternative | Monthly Cost | Why Not |
|-------------|-------------|---------|
| RDS + WAF + demo toggle | ~$11 off, ~$28 on | 5-13x more expensive, complex lifecycle management |
| RDS (public, no WAF) | ~$3 off, ~$20 on | Still requires start/stop workflow + Lambda watchdog |
| GCP Cloud Run + Neon | ~$0.30 idle, ~$18.50 on | ALB cost dominates under sustained traffic |
| EC2 nano (app + db) | ~$4.60 on, ~$1.60 off | Self-managed "pet server", operational burden |

## Consequences

- The site is always available at ~$2/month with no manual intervention
- Neon free tier limits (0.5 GB storage, 191 compute hours) are sufficient for demos but will require Pro ($19/month) if the app grows
- No WAF means relying on application-level protections — acceptable for a demo, revisit with real users
- Spotify's 25-user development limit is the main scaling constraint, not infrastructure
- App Runner's lack of GHCR support requires ECR (~$0.10/month)
- Public Neon endpoint (no VPC) is secured by TLS + strong password + random endpoint ID — equivalent or better than public RDS with security group

## Future Upgrades

| Upgrade | Trigger | Cost Impact |
|---------|---------|-------------|
| AWS WAF | Bot/abuse problems | +$8/month |
| Neon Pro | >0.5 GB data or >191 compute hours | +$19/month |
| VPC connector + NAT instance | Compliance/private networking | +$3/month |
| OpenTelemetry + Grafana Cloud | Need production observability | Free tier likely sufficient |
| Staging environment | Multiple developers | ~2x infrastructure cost |
