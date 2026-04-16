output "ecr_api_repository_url" {
  description = "ECR repository URL for the API image"
  value       = module.ecr.api_repository_url
}

output "ecr_jobs_repository_url" {
  description = "ECR repository URL for the jobs image"
  value       = module.ecr.jobs_repository_url
}

output "s3_bucket_name" {
  description = "S3 bucket name for frontend assets"
  value       = module.cdn.s3_bucket_name
}

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID"
  value       = module.cdn.distribution_id
}

output "api_lambda_function_name" {
  description = "API Lambda function name"
  value       = module.compute.api_function_name
}

output "background_lambda_function_names" {
  description = "Background job Lambda function names"
  value       = module.compute.background_function_names
}

output "api_function_url" {
  description = "API Lambda Function URL"
  value       = module.compute.api_function_url
}

output "deploy_user_arn" {
  description = "ARN of the IAM deploy user"
  value       = module.cicd.deploy_user_arn
}

output "app_url" {
  description = "Application URL"
  value       = "https://${var.domain_name}"
}
