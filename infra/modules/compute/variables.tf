variable "project" {
  description = "Project name prefix"
  type        = string
}

variable "ssm_prefix" {
  description = "SSM Parameter Store path prefix (e.g., /vibe-seeker/prod)"
  type        = string
}
