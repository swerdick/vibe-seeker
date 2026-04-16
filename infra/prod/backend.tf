terraform {
  cloud {
    organization = "vingilot"

    workspaces {
      name = "vibe-seeker-prod"
    }
  }
}

# TODO: TF_API_TOKEN (HCP Terraform user token) expires 2027-04-15. Rotate
# in GitHub Secrets and app.terraform.io before then.
