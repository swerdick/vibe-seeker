variable "domain_name" {
  description = "Full domain name for the application"
  type        = string
}

variable "subdomain" {
  description = "Subdomain portion of the domain name"
  type        = string
}

variable "cloudflare_zone_id" {
  description = "Cloudflare Zone ID for the parent domain"
  type        = string
}

variable "github_repository" {
  description = "GitHub repository in owner/repo format"
  type        = string
}

variable "project" {
  description = "Project name used as a prefix for resources"
  type        = string
  default     = "vibe-seeker"
}

variable "ssm_prefix" {
  description = "SSM Parameter Store path prefix"
  type        = string
  default     = "/vibe-seeker/prod"
}
