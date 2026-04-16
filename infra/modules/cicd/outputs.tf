output "deploy_user_name" {
  description = "Name of the IAM deploy user"
  value       = aws_iam_user.deploy.name
}

output "deploy_user_arn" {
  description = "ARN of the IAM deploy user"
  value       = aws_iam_user.deploy.arn
}
