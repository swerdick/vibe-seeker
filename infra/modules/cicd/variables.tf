variable "project" {
  description = "Project name prefix"
  type        = string
}

variable "ecr_repository_arns" {
  description = "List of ECR repository ARNs the deploy user can push to"
  type        = list(string)
}

variable "lambda_function_arns" {
  description = "List of Lambda function ARNs the deploy user can update"
  type        = list(string)
}

variable "s3_bucket_arn" {
  description = "ARN of the S3 bucket for frontend deployments"
  type        = string
}

variable "cloudfront_distribution_arn" {
  description = "ARN of the CloudFront distribution for invalidations"
  type        = string
}
