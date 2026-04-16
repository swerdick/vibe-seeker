output "api_function_name" {
  description = "Name of the API Lambda function"
  value       = aws_lambda_function.api.function_name
}

output "api_function_arn" {
  description = "ARN of the API Lambda function"
  value       = aws_lambda_function.api.arn
}

output "api_function_url" {
  description = "Lambda Function URL for the API"
  value       = aws_lambda_function_url.api.function_url
}

output "api_function_url_hostname" {
  description = "Hostname portion of the Lambda Function URL (for CloudFront origin)"
  value       = replace(replace(aws_lambda_function_url.api.function_url, "https://", ""), "/", "")
}

output "background_function_names" {
  description = "Names of background job Lambda functions"
  value       = { for k, fn in aws_lambda_function.jobs : k => fn.function_name }
}

output "all_function_arns" {
  description = "ARNs of all Lambda functions (API + background)"
  value       = concat([aws_lambda_function.api.arn], [for fn in aws_lambda_function.jobs : fn.arn])
}
