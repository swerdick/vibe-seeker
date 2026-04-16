# ADR 001: Deployment Architecture

**Status:** Amended
**Date:** 2026-04-15 (originally 2026-03-27)

**Amendment 2 (2026-04-15):** Switched DNS from Route 53 to Cloudflare (domain already hosted there), Terraform state from S3+DynamoDB bootstrap to HCP Terraform free tier, and domain to `vibeseeker.vingilot.dev` (subdomain of existing domain). Neon database managed externally (not in Terraform). AWS auth uses static keys initially (OIDC planned).

**Amendment 1 (2026-04-05):** Migrated backend compute from App Runner to Lambda + Function URL. AWS announced App Runner deprecation on 2026-03-31 (no new customers after 2026-04-30, maintenance mode for existing customers). See [Alternatives Considered](#alternatives-considered) for migration analysis.

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
- CloudFront natively supports path-based routing to multiple origins (S3 + Lambda Function URL) without requiring a load balancer
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
[S3]       [Lambda Function URL]

 [EventBridge Scheduler]
         |
  [Lambda: sync-events]  ──→  Neon
  [Lambda: sync-venues]  ──→  Neon
```

Single CloudFront distribution with path-based routing:
- Default behavior (`/*`): S3 origin via Origin Access Control (private bucket)
- Ordered behavior (`/api/*`): Lambda Function URL custom origin

A CloudFront Function rewrites non-file URIs to `/index.html` for SPA routing (mirrors nginx `try_files`). The `/api/*` behavior uses `AllViewerExceptHostHeader` origin request policy to forward session cookies to the Lambda origin. The default S3 behavior uses `CachingOptimized` which strips cookies.

This single-domain setup eliminates CORS issues and keeps the Spotify OAuth redirect URI stable.

### Backend: Lambda + Function URL

The Go backend runs on Lambda using the [AWS Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter), which proxies Lambda events to the existing HTTP server with minimal code changes. The container image is pulled from ECR.

- **Function URL** provides a public HTTPS endpoint used as a CloudFront custom origin — no ALB or API Gateway required
- **Lambda Web Adapter** runs as a Lambda extension layer, allowing the standard Go `net/http` server to handle requests unmodified
- Runtime secrets read from SSM Parameter Store at cold start via the AWS SDK
- Cost: effectively $0/month at demo traffic levels (free tier: 1M requests + 400K GB-seconds/month)
- Cold starts: ~500ms-1s after idle period; Go's fast startup keeps this acceptable

### Background Jobs: EventBridge + Lambda

Background sync tasks (event ingestion, venue updates) run as separate Lambda functions triggered by EventBridge Scheduler rules. These were designed from the start to be separable from the API server.

- Each background job is its own Lambda function with its own IAM role and schedule
- Connects to Neon directly over the public internet — no VPC or NAT Gateway needed (same as the API Lambda)
- EventBridge Scheduler is free for the first 14M invocations/month
- 15-minute Lambda timeout is sufficient — current sync jobs complete in under 5 minutes
- If a job ever exceeds 15 minutes, it can be migrated to an ECS Scheduled Task

### Database: Neon (Serverless Postgres)

Neon over RDS because:
- **Scales to zero automatically** — compute suspends after ~5 min idle, wakes in ~500ms on connection
- **Free tier** — 0.5 GB storage, 191 compute hours/month (sufficient for a demo app)
- **No lifecycle management** — no start/stop workflow, no 7-day auto-restart problem, no Lambda watchdog
- **No VPC complexity** — RDS in a VPC would require a NAT Gateway (~$32/month) for Lambda to reach external APIs (Spotify, Last.fm, Ticketmaster). Public RDS was the alternative, but Neon's managed proxy with TLS + random endpoint IDs is more secure. Lambda without a VPC connects to Neon's public endpoint directly.
- **Portable** — standard `pg_dump` for backups, `pgx` driver works without code changes
- **Terraform-managed** — `kislerdm/neon` provider creates project, database, role, and endpoint

Neon project runs in `aws-us-east-1` to minimize latency to Lambda (us-east-1). The pooled connection endpoint (PgBouncer) is used by default.

RDS `db.t4g.micro` would have cost ~$14/month running + $2.30/month stopped (storage), plus required a Lambda + EventBridge auto-re-stop mechanism for the 7-day restart problem. Neon eliminates all of this at $0/month.

### Container Registry: ECR

Lambda container images are pulled from ECR. ECR costs ~$0.10/month for image storage with a lifecycle policy keeping the last 5 images.

### WAF: Not Initially

AWS WAF costs $8/month ($5 web ACL + $1/rule) — more than all other services combined for a demo app. Sufficient free-tier protection exists:

1. Shield Standard (free with CloudFront) — network-layer DDoS
2. Lambda concurrency limits — reserved concurrency caps compute cost under sustained attack
3. Authenticated endpoints — most `/api/*` routes require valid JWT, bots get 401
4. Turnstile — anonymous auth requires valid bot-detection token
5. Application-level rate limiting (`internal/ratelimit/`)
6. `robots.txt` + `noindex` meta tags

### CI/CD: GitHub Actions

GitHub Actions authenticates to AWS via static IAM credentials (stored as GitHub secrets `AWS_ACCESS_KEY`/`AWS_SECRET_KEY`). Migration to OIDC federation is planned. A `vibe-seeker-deploy` IAM user with least-privilege permissions:
- ECR: authenticate + push images
- Lambda: update function code
- S3: sync frontend assets
- CloudFront: create invalidations

Terraform Cloud authenticates via client credentials (`TERRAFORM_CLOUD_CLIENT_ID`/`TERRAFORM_CLOUD_CLIENT_SECRET`).

Three deploy workflows:
- `deploy-frontend.yml` — on push to main (`frontend/**`): build, S3 sync, CloudFront invalidation
- `deploy-backend.yml` — on push to main (`backend/**`): build container, push to ECR, update Lambda function image
- `tf-plan-apply.yml` — on PR (`infra/**`): plan + PR comment; on merge: apply

### Secrets Management: SSM Parameter Store

Runtime secrets (API keys, JWT secret, DATABASE_URL) stored in SSM Parameter Store as SecureString. Lambda functions read SSM parameters at cold start via the AWS SDK, caching values for the lifetime of the execution environment.

Build-time values (deploy role ARN, bucket names, Turnstile site key) stored as GitHub repository variables.

SSM hierarchy: `/vibe-seeker/{env}/{secret-name}`

DATABASE_URL is auto-populated by Terraform from Neon module outputs. All other secrets are created with placeholder values (`lifecycle { ignore_changes = [value] }`) and populated manually via `aws ssm put-parameter`. Each Lambda function's IAM role grants `ssm:GetParameter` only for the specific parameters it needs.

### DNS: Cloudflare

Domain `vibeseeker.vingilot.dev` is a subdomain of `vingilot.dev`, which is already hosted on Cloudflare. DNS records (ACM validation CNAMEs + app CNAME → CloudFront) are managed by Terraform using the `cloudflare/cloudflare` provider. No Route 53 hosted zone needed — saves $0.50/month.

ACM certificate for `vibeseeker.vingilot.dev` with DNS validation via Cloudflare records.

## Infrastructure as Code

### Terraform Structure

```
infra/
├── modules/
│   ├── cdn/          # S3 + CloudFront + SPA rewrite function
│   ├── compute/      # Lambda functions + Function URLs + IAM roles + EventBridge schedules
│   ├── ecr/          # Container registry + lifecycle policy
│   └── cicd/         # Deploy IAM user + policy (OIDC planned)
└── prod/             # Production environment composition
    ├── main.tf       # Wires modules + ACM cert + Cloudflare DNS + SSM parameters
    ├── variables.tf
    ├── outputs.tf
    ├── backend.tf    # HCP Terraform cloud{} block
    ├── providers.tf  # AWS + Cloudflare provider config
    └── terraform.tfvars
```

Neon database is managed externally (created via Neon console, connection string stored in SSM). No `database/` module — avoids dependency on the community `kislerdm/neon` provider. No `bootstrap/` directory — HCP Terraform manages state.

### Environment-Specific Values

| Layer | Tool | Contents |
|-------|------|----------|
| Infrastructure config | `terraform.tfvars` per env | Instance sizes, domain name, Neon region |
| Application secrets | AWS SSM Parameter Store | API keys, JWT secret, DATABASE_URL |
| Build-time config | GitHub Actions variables | Turnstile site key, deploy role ARN, bucket names |

## Cost

### Monthly: ~$0.11

| Service | Cost | Notes |
|---------|------|-------|
| Lambda (API + background jobs) | $0.00 | Free tier covers demo traffic (1M req + 400K GB-sec/month) |
| EventBridge Scheduler | $0.00 | Free for first 14M invocations/month |
| Cloudflare DNS | $0.00 | Included with existing domain |
| ECR | ~$0.10 | Container image storage |
| S3 + CloudFront | ~$0.01 | Free tier covers demo traffic |
| Neon | $0.00 | Free tier |
| SSM | $0.00 | Standard parameters free |
| HCP Terraform | $0.00 | Free tier (500 resources) |

Domain: `vibeseeker.vingilot.dev` — subdomain of existing `vingilot.dev`, no additional registration cost.

Note: Lambda free tier is permanent (not 12-month limited). At demo traffic levels (~100 req/day), Lambda compute usage is ~18.75 GB-seconds/month — well under the 400,000 GB-second free tier. Even without the free tier, compute cost would be under $0.01/month.

### Alternatives Considered

| Alternative | Monthly Cost | Why Not |
|-------------|-------------|---------|
| App Runner (original) | ~$1.58 idle | **Deprecated.** No new customers after 2026-04-30, no new features. Was the previous choice — worked well but AWS is sunsetting it. |
| ECS Express Mode | ~$52+ | AWS's recommended App Runner replacement, but auto-provisions an ALB (~$16/month minimum). Designed for production services, not hobby projects. |
| CloudFront → EC2 t4g.nano | ~$7.36 | $3.07 compute + $3.65 public IPv4 charge (introduced Feb 2024) + $0.64 EBS. 3.5x more expensive than App Runner was. Requires patching. |
| ECS Fargate (min, no ALB) | ~$12.66 | Ephemeral public IP changes on every task restart, breaking CloudFront origin. ALB would fix this but costs ~$16/month. |
| RDS + WAF + demo toggle | ~$11 off, ~$28 on | 5-13x more expensive, complex lifecycle management |
| GCP Cloud Run + Neon | ~$0.30 idle, ~$18.50 on | ALB cost dominates under sustained traffic |

## Consequences

- The site is always available at under $1/month with no manual intervention
- Cold starts (~500ms-1s) are the main tradeoff vs. App Runner's always-on model — acceptable for a demo app
- Background sync jobs must complete within Lambda's 15-minute timeout — current jobs run under 5 minutes, but this needs monitoring
- Lambda Web Adapter adds a dependency but avoids rewriting the Go backend to Lambda-native handlers
- Neon free tier limits (0.5 GB storage, 191 compute hours) are sufficient for demos but will require Pro ($19/month) if the app grows
- No WAF means relying on application-level protections — acceptable for a demo, revisit with real users
- Spotify's 25-user development limit is the main scaling constraint, not infrastructure
- Lambda container images require ECR (~$0.10/month)
- Public Neon endpoint (no VPC) is secured by TLS + strong password + random endpoint ID — Lambda connects directly without VPC or NAT Gateway

## Future Upgrades

| Upgrade | Trigger | Cost Impact |
|---------|---------|-------------|
| AWS WAF | Bot/abuse problems | +$8/month |
| Neon Pro | >0.5 GB data or >191 compute hours | +$19/month |
| ECS Scheduled Task | Background job exceeds 15-minute Lambda timeout | ~$1-2/month per job |
| Provisioned concurrency | Cold starts unacceptable for user-facing requests | ~$3-5/month |
| OpenTelemetry + Grafana Cloud | Need production observability | Free tier likely sufficient |
| Staging environment | Multiple developers | ~2x infrastructure cost |
