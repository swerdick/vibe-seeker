terraform {
  cloud {
    organization = "vingilot"

    workspaces {
      name = "vibe-seeker-prod"
    }
  }
}
