terraform {
  required_providers {
    anthropic = {
      source  = "anthropics-contrib/anthropic"
      version = "0.1.0"
    }
  }
}

provider "anthropic" {}

# Vault grouping related credentials.
resource "anthropic_managed_vault" "integrations" {
  display_name = "third-party-integrations"

  metadata = {
    managed_by = "terraform"
    team       = "platform"
  }
}

# MCP OAuth credential for GitHub (access token only).
resource "anthropic_managed_vault_credential" "github" {
  vault_id     = anthropic_managed_vault.integrations.id
  display_name = "GitHub OAuth"

  auth = {
    type           = "mcp_oauth"
    mcp_server_url = "https://mcp.example.com/github"
    access_token   = var.github_access_token
  }
}

# Another credential for a different integration.
resource "anthropic_managed_vault_credential" "jira" {
  vault_id     = anthropic_managed_vault.integrations.id
  display_name = "Jira OAuth"

  auth = {
    type           = "mcp_oauth"
    mcp_server_url = "https://mcp.example.com/jira"
    access_token   = var.jira_access_token
  }
}

variable "github_access_token" {
  type      = string
  sensitive = true
}

variable "jira_access_token" {
  type      = string
  sensitive = true
}

output "vault_id" {
  value = anthropic_managed_vault.integrations.id
}

output "credential_type" {
  value = anthropic_managed_vault_credential.github.credential_type
}
