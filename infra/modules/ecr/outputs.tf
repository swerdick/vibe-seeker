output "api_repository_url" {
  description = "ECR repository URL for the API image"
  value       = aws_ecr_repository.api.repository_url
}

output "jobs_repository_url" {
  description = "ECR repository URL for the jobs image"
  value       = aws_ecr_repository.jobs.repository_url
}

output "api_repository_arn" {
  description = "ECR repository ARN for the API image"
  value       = aws_ecr_repository.api.arn
}

output "jobs_repository_arn" {
  description = "ECR repository ARN for the jobs image"
  value       = aws_ecr_repository.jobs.arn
}
