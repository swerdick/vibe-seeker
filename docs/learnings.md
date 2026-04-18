# Learnings

Things I learned building Vibe Seeker that I wish I'd known before starting.

## Data Quality is an Issue

Documentation describes an idealized version of the API. The actual data tells a different story.

**Spotify deprecated genre data mid-build.** The `genres` field on artist objects — documented, still present in responses 
— returns empty arrays. No announcement in the changelog at the time. I found out when my taste vectors came back empty. 
The fix was pivoting to Last.fm's community-sourced tags, which turned out to be *better* than Spotify's genres anyway 
(richer taxonomy, mood/vibe tags, not just genre labels).

**Ticketmaster's venue data is full of ghosts.** A geo search for NYC venues returns ~945 results. Only 55 have active events. 
The rest are defunct venues (closed years ago), non-music spaces (conference centers, cruise ships, schools), and duplicate 
entries. Webster Hall — one of NYC's most active music venues — appeared in our DB under an inactive venue ID with zero 
events, while the active ID wasn't returned by the geo search at all. Some international venues (Spain, Mexico) appeared
in NYC results due to bad geocoding in Ticketmaster's database.

**Rate limits don't always match documentation.** Ticketmaster documents 5 req/sec, but we hit 429s well before that 
threshold when fetching events for 900+ venues sequentially. The solution was adaptive rate limiting: start at 5 req/sec, 
back off by 1 second on every 429, speed back up on success. Simple but necessary.

## The Data Source Landscape Is Hostile to Hobbyists

Before building, I assumed I'd have 3-4 event data sources to pull from. The reality:

| Source           | Status  | Why                                                               |
|------------------|---------|-------------------------------------------------------------------|
| Ticketmaster     | Works   | Free tier, decent API, data requires de-dupes and double checking |
| Last.fm          | Works   | Free, great tag data, rate limited                                |
| SeatGeek         | Pending | Developer account approval required (still pending)               |
| Bandsintown      | Blocked | API restricted to artist managers only                            |
| Songkick         | Blocked | $500 per million requests                                         |
| Resident Advisor | Blocked | Returns 403 to any automated request                              |

The small venue data that would make this app truly special (DIY spaces, basement shows, $10 cover charges) is exactly
the data that's hardest to get programmatically. The venues that need discovery the most are the ones least likely to be 
in any API

## Cache Everything

The read-through cache for Last.fm tags transformed the app from "unusably slow" to "fast on repeat visits."

First vibe sync: ~4 minutes (1,200 Last.fm API calls at 200ms each). Second sync: ~5 seconds (85% cache hits, only ~170 
API calls for new artists). The cache is a PostgreSQL table with a 15-day TTL — simple, durable, and doubles as integration
test fixtures

The same cache is shared between user vibe computation and venue vibe computation, so an artist who appears in both your
Spotify top 50 and a venue's show history only gets looked up once.


## Multi-Source Data Requires Normalization at Every Layer

When you pull data from Spotify, Last.fm, and Ticketmaster, nothing agrees:

- **Artist names**: Ticketmaster sends "RÜFÜS DU SOL" (with umlauts). Last.fm only recognizes "RUFUS DU SOL." Solution:
  diacritics stripping as a retry strategy
- **Genre taxonomy**: Last.fm uses fine-grained community tags ("dream pop", "shoegaze"). Ticketmaster uses a coarse hierarchy
  ("Music > Rock > Alternative Rock"). Solution: normalize everything to lowercase, weight Ticketmaster's broad genres lower
  than Last.fm's specific tags
- **Venue identity**: Ticketmaster has duplicate venue entries for the same physical location with different IDs. Solution:
  source-prefixed IDs (`tm_`) and accepting that some duplicates slip through.
  - As data sources expand I'll need to try to de-dupe by things like addresses, and normalized venue names, and lat/long
    if available
- **Data freshness**: Spotify tokens expire hourly. Last.fm tags are stable for weeks (or probably more). Ticketmaster events
  change daily. Solution: different TTLs for different data types (tokens: refresh on use, tags: 15 days, venues: 6 hours)

## HCP Terraform Is Not Worth It for Small Projects

We originally chose HCP Terraform (formerly Terraform Cloud) for state management because the ADR said it would be
"simpler than S3+DynamoDB bootstrap." It was not.

**The UX is split across two portals.** Organization management is at `portal.cloud.hashicorp.com`. Workspaces and runs
are at `app.terraform.io`. These are supposedly the same product but navigating between them is confusing — links
between portals don't always land where you expect, and features are split inconsistently.

**Three different credential types.** HCP service principal credentials (client ID + client secret) are *not* the same as
Terraform Cloud API tokens, even though both are created under the same umbrella product. The `hashicorp/setup-terraform`
GitHub Action only accepts API tokens, not service principal credentials. We created credentials we couldn't use before
discovering this.

**Workspace variables duplicate GitHub secrets.** Because `terraform plan/apply` runs remotely in HCP Terraform, provider
credentials (AWS keys, Cloudflare token) must be configured *both* as GitHub secrets (for non-Terraform workflows) *and*
as HCP Terraform workspace variables. Double the credential management surface.

**The fix: S3 backend.** For a single-developer project, an S3 bucket with versioning and encryption is simpler, cheaper
($0/month at this scale), and eliminates an entire service dependency. The bucket is private with AES-256 encryption,
public access blocked at every level, and versioning enabled for state recovery. DynamoDB locking is skipped in favor
of a GitHub Actions `concurrency` group on the Terraform workflow — overlapping applies are serialized at the workflow
level, which is sufficient for a single-developer setup where all applies flow through CI.

## Lambda Container Images Can't Come From Public ECR

`aws_lambda_function` with `package_type = "Image"` rejects images from the public ECR gallery. I tried using
`public.ecr.aws/lambda/provided:al2023` as a throwaway placeholder so Terraform could create the function before CI had
pushed a real image. AWS returned `InvalidParameterValueException: Source image ... is not valid`. The image must live
in a private ECR repo in the same AWS account as the function.

This creates a chicken-and-egg for fresh deployments: the Lambda can't be created without an image, but CI can't push
the image until ECR exists, and you'd like the whole thing to come up in one `terraform apply`. The fix was a one-time
manual seed push before the first Lambda-creating apply: build the real image locally, push it to the just-created ECR
repo, then re-apply. Subsequent deploys go through CI as intended.

If I did this again I'd build this into the Terraform as a `null_resource` with a `local-exec` that runs `docker build`
and `docker push` against the ECR output URL, gated on `terraform apply` the first time. But for a one-off bootstrap,
the manual seed push was fine.

## Terraform Provider Major Versions Quietly Rename Resources

Upgrading to Cloudflare provider v5 renamed `cloudflare_record` to `cloudflare_dns_record`. The attribute model also
changed (`value` became `content`, `hostname` was removed as a computed attribute, etc.). None of this was surfaced
until `terraform plan` failed with "The provider cloudflare/cloudflare does not support resource type
`cloudflare_record`" — which is accurate but doesn't point at the rename.

The takeaway: major provider upgrades aren't drop-in. Lockfile commits (`.terraform.lock.hcl`) protect you from *surprise*
upgrades, but once you intentionally bump a major version, budget time for resource renames and attribute migrations.
Check the provider's upgrade guide in the Terraform registry before bumping.

## `continue-on-error` Quietly Hides Real Failures

The Terraform plan step in our CI workflow originally had `continue-on-error: true` so the "Comment PR" step downstream
could still post the plan output regardless of exit code. Side effect: when `terraform plan` failed, the job reported
*succeeded* in the GitHub Actions UI. The real error was only visible if you clicked into the step logs.

The fix was to remove `continue-on-error` and gate the comment step on `if: always() && steps.plan.outcome != 'skipped'`.
That way the plan step fails honestly, but the comment still posts so the PR author sees what broke.

General rule: `continue-on-error: true` should almost always be paired with an explicit success/failure check later in
the job, or replaced with `if: always()` on the dependent steps. Otherwise it's a silent "green even when red" switch.

## Opt-Out Feature Flags Default to the Unsafe State

Our sync buttons — which trigger expensive background jobs — were originally gated on
`VITE_SHOW_SYNC_CONTROLS !== "false"`. The intent was "hide in prod, show in dev." The actual behavior was "show whenever
the variable isn't literally the string 'false'," which includes the very common case of the variable being unset.
Production deploys don't ship a `.env` file, so the buttons were visible in prod until Copilot flagged it during review.

Flipping to `=== "true"` makes the safe state the default — controls are hidden unless an explicit opt-in fires. Same
principle as `NODE_ENV === "development"` instead of `NODE_ENV !== "production"`: opt-in gates protect you from the
misconfigured case, opt-out gates don't.

Bonus gotcha: the test suite passed locally because my local `.env` set the flag, but failed in CI where no `.env`
exists. The fix was to stub the env var in `test-setup.ts` so all tests see the authenticated-dev UX, which is what the
tests are actually asserting about.

## Building Lambda amd64 Images On Apple Silicon Is Slow

First seed push for this project: `podman build --platform linux/amd64` of the Go backend took ~8 minutes per image on
an M-series Mac. Native Go build in the same container on amd64 hardware is under 30 seconds. The difference is QEMU
user-mode emulation, which translates every amd64 instruction into ARM64 at runtime.

Two ways out:
- **Do it once, do it in CI forever after.** GitHub Actions runners are native amd64. Local seed pushes are the only
  time this matters; subsequent deploys happen in CI and complete in a minute or two.
- **Switch Lambda to arm64.** AWS Lambda supports `architectures = ["arm64"]`, which is ~20% cheaper per GB-second *and*
  builds natively on M-series Macs. For greenfield projects on Apple Silicon, arm64 Lambda should probably be the
  default unless you have a dependency that doesn't ship ARM binaries.

## AWS IAM for Terraform Bootstrap Is Non-Obvious

The first Terraform apply failed with `AccessDenied` on a growing list of services. Attaching a single AWS-managed
policy wasn't enough — our modules touch a surprisingly wide spread of services. The final list that made the apply
succeed on the `gh-actions` bootstrap user was:

- `AmazonEC2ContainerRegistryFullAccess` — ECR repos + lifecycle policies
- `AmazonS3FullAccess` — frontend bucket + the state bucket
- `CloudFrontFullAccess` — distribution + OAC + CloudFront Functions
- `AWSLambda_FullAccess` — API and background job functions
- `AmazonSSMFullAccess` — parameter store for runtime secrets
- `IAMFullAccess` — deploy user + Lambda execution roles
- `AWSCertificateManagerFullAccess` — the ACM cert for the custom domain
- `AmazonEventBridgeSchedulerFullAccess` — background job cron schedules (this is a **separate service** from
  EventBridge; `AmazonEventBridgeFullAccess` does not cover the `scheduler:*` API)

The split between EventBridge and EventBridge Scheduler is the most confusing one — they share a product name in the
AWS console but are distinct APIs with distinct IAM actions. Worth budgeting an iteration or two to discover what's
missing when bootstrapping a new account: Terraform will tell you exactly which action is denied, but it'll only tell
you one at a time per apply.

## CloudFront OAC for Lambda URLs Has Underdocumented Requirements

CloudFront Origin Access Control (OAC) lets you restrict a Lambda Function URL so only CloudFront can invoke it — no
one can bypass CloudFront by hitting the Lambda URL directly. The concept is sound and the security benefit is real, but
the setup has several gotchas that cost us hours of debugging.

**You need TWO Lambda permissions, not one.** The AWS docs bury this in a code example: CloudFront OAC requires both
`lambda:InvokeFunctionUrl` AND `lambda:InvokeFunction` permissions on the Lambda function. Every Terraform example and
blog post I found only showed the first one. With only `InvokeFunctionUrl`, CloudFront's signed requests are rejected
with a 403 `AccessDeniedException`. The error message doesn't hint at the missing permission — it just says "Forbidden."

```hcl
# Both are required. Missing the second one causes silent 403s.
resource "aws_lambda_permission" "cloudfront_invoke_url" {
  action                 = "lambda:InvokeFunctionUrl"
  function_name          = aws_lambda_function.api.function_name
  principal              = "cloudfront.amazonaws.com"
  source_arn             = aws_cloudfront_distribution.this.arn
  function_url_auth_type = "AWS_IAM"
}

resource "aws_lambda_permission" "cloudfront_invoke_function" {
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api.function_name
  principal     = "cloudfront.amazonaws.com"
  source_arn    = aws_cloudfront_distribution.this.arn
}
```

**POST and PUT requests require `x-amz-content-sha256` from the browser.** CloudFront OAC signs requests with SigV4,
and Lambda validates the signature. For POST/PUT requests, Lambda requires the body to be included in the signature via
a SHA256 hash in the `x-amz-content-sha256` header. The gotcha: this header must be sent by the *client* (browser), not
added by CloudFront. CloudFront includes whatever the viewer sends in its SigV4 computation.

This means your frontend JavaScript needs to compute `SHA256(request_body)` for every POST request and include it as a
header. For empty-body POSTs, it's a constant (`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`).
For POSTs with bodies, you compute it dynamically:

```js
const encoded = new TextEncoder().encode(body);
const digest = await crypto.subtle.digest("SHA-256", encoded);
const hash = Array.from(new Uint8Array(digest)).map(b => b.toString(16).padStart(2, "0")).join("");
```

We centralized this into a shared `post()` helper in `frontend/src/utils/api.ts`.

Without this header, GET requests work fine (no body to hash) but every POST returns 403. The error message is the same
generic "Forbidden" with no mention of the missing hash.

**The Lambda URL origin needs `custom_origin_config`.** Unlike S3 origins (which CloudFront auto-detects), Lambda
Function URL origins are custom origins and require explicit `custom_origin_config` with `origin_protocol_policy =
"https-only"`. Removing it (as I tried while debugging the 403) causes CloudFront to treat the Lambda URL domain as an
S3 bucket, failing with "The parameter Origin DomainName does not refer to a valid S3 bucket." The OAC attaches
alongside `custom_origin_config` — they're not mutually exclusive.

**Debugging is hard.** Direct SigV4 curl to the Lambda URL works (`curl --aws-sigv4`), but CloudFront → Lambda returns
403 with no actionable details. Standard CloudFront logs don't include auth headers. The only diagnostic that helped was
checking the Lambda resource policy (`aws lambda get-policy`) and comparing it to the AWS docs' example — that's how we
found the missing `InvokeFunction` permission. For the content hash issue, we found it in a single sentence in the docs
that's easy to miss.

## The Algorithm Is Simple — The Data Pipeline Is Hard

The actual matching math is ~30 lines of code (cosine similarity on two maps). The data pipeline to get clean, normalized, 
cached tag vectors from three different APIs with different auth models, rate limits, and data quality issues is ~2,000 
lines. Getting the data right is 80% of the work
