terraform {
  backend "s3" {
    bucket = "vibe-seeker-tfstate"
    key    = "prod/terraform.tfstate"
    region = "us-east-1"
  }
}
