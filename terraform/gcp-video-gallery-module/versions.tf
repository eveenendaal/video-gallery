terraform {
  required_providers {
    google = {
      source = "hashicorp/google"
      version = "4.69.1"
    }
    random = {
      source = "hashicorp/random"
      version = ">= 3.1.0"
    }
  }
  required_version = ">= 1.4.0"
}
