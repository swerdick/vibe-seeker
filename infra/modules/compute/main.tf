# --- API Lambda ---

resource "aws_iam_role" "api_lambda" {
  name = "${var.project}-api-lambda"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "api_lambda" {
  name = "${var.project}-api-lambda"
  role = aws_iam_role.api_lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "arn:aws:logs:*:*:*"
      },
      {
        Effect   = "Allow"
        Action   = "ssm:GetParametersByPath"
        Resource = "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter${var.ssm_prefix}/*"
      },
    ]
  })
}

resource "aws_lambda_function" "api" {
  function_name = "${var.project}-api"
  role          = aws_iam_role.api_lambda.arn
  package_type = "Image"
  # image_uri must point to a real ECR image in the same account.  A seed
  # image is pushed once during bootstrap; after that, lifecycle.ignore_changes
  # lets the deploy-backend workflow update it with aws lambda update-function-code.
  image_uri   = "${var.api_ecr_repository_url}:latest"
  timeout     = 30
  memory_size = 256

  reserved_concurrent_executions = 10

  environment {
    variables = {
      AWS_LWA_PORT = "8080"
      SSM_PREFIX   = var.ssm_prefix
    }
  }

  lifecycle {
    ignore_changes = [image_uri]
  }
}

resource "aws_lambda_function_url" "api" {
  function_name      = aws_lambda_function.api.function_name
  authorization_type = "AWS_IAM"
}

# --- Background Job Lambda ---

resource "aws_iam_role" "jobs_lambda" {
  name = "${var.project}-jobs-lambda"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "jobs_lambda" {
  name = "${var.project}-jobs-lambda"
  role = aws_iam_role.jobs_lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "arn:aws:logs:*:*:*"
      },
      {
        Effect   = "Allow"
        Action   = "ssm:GetParametersByPath"
        Resource = "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter${var.ssm_prefix}/*"
      },
    ]
  })
}

locals {
  background_jobs = {
    sync-venues      = "rate(1 day)"
    sync-venue-vibes = "rate(1 day)"
    tag-enrichment   = "rate(7 days)"
  }
}

resource "aws_lambda_function" "jobs" {
  for_each = local.background_jobs

  function_name = "${var.project}-${each.key}"
  role          = aws_iam_role.jobs_lambda.arn
  package_type = "Image"
  image_uri    = "${var.jobs_ecr_repository_url}:latest"
  timeout      = 900
  memory_size  = 512

  environment {
    variables = {
      JOB_NAME   = each.key
      SSM_PREFIX = var.ssm_prefix
    }
  }

  lifecycle {
    ignore_changes = [image_uri]
  }
}

# --- EventBridge Scheduler ---

resource "aws_scheduler_schedule_group" "this" {
  name = var.project
}

resource "aws_iam_role" "scheduler" {
  name = "${var.project}-scheduler"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = { Service = "scheduler.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "scheduler" {
  name = "${var.project}-scheduler"
  role = aws_iam_role.scheduler.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "lambda:InvokeFunction"
      Resource = [for fn in aws_lambda_function.jobs : fn.arn]
    }]
  })
}

resource "aws_scheduler_schedule" "jobs" {
  for_each = local.background_jobs

  name       = "${var.project}-${each.key}"
  group_name = aws_scheduler_schedule_group.this.name
  state      = "ENABLED"

  flexible_time_window {
    mode                      = "FLEXIBLE"
    maximum_window_in_minutes = 30
  }

  schedule_expression = each.value

  target {
    arn      = aws_lambda_function.jobs[each.key].arn
    role_arn = aws_iam_role.scheduler.arn
  }
}

# --- Data Sources ---

data "aws_region" "current" {}
data "aws_caller_identity" "current" {}
