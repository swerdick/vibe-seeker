variable "project" {
  description = "Project name prefix"
  type        = string
}

variable "domain_name" {
  description = "Custom domain name for the CloudFront distribution"
  type        = string
}

variable "acm_certificate_arn" {
  description = "ARN of the ACM certificate for HTTPS"
  type        = string
}

variable "lambda_function_url_hostname" {
  description = "Hostname of the Lambda Function URL (without https://)"
  type        = string
}
