variable "project" {
  description = "Project name prefix"
  type        = string
}

variable "ssm_prefix" {
  description = "SSM Parameter Store path prefix (e.g., /vibe-seeker/prod)"
  type        = string
}

variable "api_ecr_repository_url" {
  description = "ECR repository URL for the API image"
  type        = string
}

variable "jobs_ecr_repository_url" {
  description = "ECR repository URL for the background jobs image"
  type        = string
}
