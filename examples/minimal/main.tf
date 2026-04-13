terraform {
  required_providers {
    anthropic = {
      source  = "anthropics-contrib/anthropic"
      version = "0.1.0"
    }
  }
}

provider "anthropic" {}

# Minimal environment with unrestricted networking.
resource "anthropic_managed_environment" "sandbox" {
  name = "sandbox"

  config {
    type = "cloud"
    networking {
      type = "unrestricted"
    }
  }
}

# Minimal agent with just a model.
resource "anthropic_managed_agent" "assistant" {
  name     = "assistant"
  model_id = "claude-sonnet-4-6"
}
