terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.42"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"

  default_tags {
    tags = {
      Project     = "vibe-seeker"
      Environment = "prod"
      ManagedBy   = "terraform"
    }
  }
}

provider "cloudflare" {
  # Reads CLOUDFLARE_API_TOKEN from environment
}
