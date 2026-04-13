terraform {
  required_providers {
    anthropic = {
      source  = "anthropics-contrib/anthropic"
      version = "0.1.0"
    }
  }
}

provider "anthropic" {
  archive_on_destroy = false
}

# Locked-down environment with limited networking and pre-installed packages.
resource "anthropic_managed_environment" "production" {
  name        = "production-env"
  description = "Hardened environment with allowlisted hosts only"

  metadata = {
    compliance = "soc2"
    team       = "security"
  }

  config = {
    type = "cloud"

    networking = {
      type                   = "limited"
      allow_mcp_servers      = true
      allow_package_managers = true
      allowed_hosts = [
        "api.github.com",
        "registry.npmjs.org",
        "pypi.org",
        "files.pythonhosted.org",
      ]
    }

    packages = {
      type = "packages"
      pip  = ["requests==2.32.3", "pydantic==2.11.0"]
      npm  = ["typescript@5.8.3", "eslint@9.28.0"]
      apt  = ["git", "curl", "jq"]
      go   = ["golang.org/x/tools@latest"]
    }
  }
}

output "environment_id" {
  value = anthropic_managed_environment.production.id
}
