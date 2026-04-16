# --- ACM Certificate ---
# Inlined here (not a module) to break the circular dependency between
# certificate validation (needs Cloudflare) and CDN (needs cert ARN),
# while the DNS alias record needs the CloudFront domain name.

resource "aws_acm_certificate" "app" {
  domain_name       = var.domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "cloudflare_dns_record" "acm_validation" {
  for_each = {
    for dvo in aws_acm_certificate.app.domain_validation_options : dvo.domain_name => {
      name  = dvo.resource_record_name
      type  = dvo.resource_record_type
      value = dvo.resource_record_value
    }
  }

  zone_id = var.cloudflare_zone_id
  name    = each.value.name
  type    = each.value.type
  content = each.value.value
  ttl     = 300
  proxied = false
}

resource "aws_acm_certificate_validation" "app" {
  certificate_arn         = aws_acm_certificate.app.arn
  validation_record_fqdns = [for dvo in aws_acm_certificate.app.domain_validation_options : dvo.resource_record_name]
}

# --- Infrastructure Modules ---

module "ecr" {
  source  = "../modules/ecr"
  project = var.project
}

module "compute" {
  source = "../modules/compute"

  project    = var.project
  ssm_prefix = var.ssm_prefix
}

module "cdn" {
  source = "../modules/cdn"

  project                      = var.project
  domain_name                  = var.domain_name
  acm_certificate_arn          = aws_acm_certificate_validation.app.certificate_arn
  lambda_function_url_hostname = module.compute.api_function_url_hostname
}

module "cicd" {
  source = "../modules/cicd"

  project                     = var.project
  ecr_repository_arns         = [module.ecr.api_repository_arn, module.ecr.jobs_repository_arn]
  lambda_function_arns        = module.compute.all_function_arns
  s3_bucket_arn               = module.cdn.s3_bucket_arn
  cloudfront_distribution_arn = module.cdn.distribution_arn
}

# --- DNS Alias Record ---

resource "cloudflare_dns_record" "app" {
  zone_id = var.cloudflare_zone_id
  name    = var.subdomain
  type    = "CNAME"
  content = module.cdn.distribution_domain_name
  ttl     = 1 # Auto
  proxied = false
}

# --- SSM Parameters ---
# Parameters with known values are set directly.
# Secret parameters are created with placeholder values; populate manually
# via `aws ssm put-parameter --overwrite`.

locals {
  ssm_known = {
    SPOTIFY_REDIRECT_URI = "https://${var.domain_name}/api/auth/callback"
    FRONTEND_URL         = "https://${var.domain_name}"
    CORS_ORIGIN          = "https://${var.domain_name}"
    ENVIRONMENT          = "production"
  }

  ssm_secrets = [
    "DATABASE_URL",
    "SPOTIFY_CLIENT_ID",
    "SPOTIFY_CLIENT_SECRET",
    "JWT_SECRET",
    "LASTFM_API_KEY",
    "TICKETMASTER_CONSUMER_KEY",
    "TURNSTILE_SECRET_KEY",
  ]
}

resource "aws_ssm_parameter" "known" {
  for_each = local.ssm_known

  name  = "${var.ssm_prefix}/${each.key}"
  type  = "String"
  value = each.value
}

resource "aws_ssm_parameter" "secrets" {
  for_each = toset(local.ssm_secrets)

  name  = "${var.ssm_prefix}/${each.key}"
  type  = "SecureString"
  value = "PLACEHOLDER"

  lifecycle {
    ignore_changes = [value]
  }
}
