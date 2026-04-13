terraform {
  required_providers {
    anthropic = {
      source  = "anthropics-contrib/anthropic"
      version = "0.1.0"
    }
  }
}

provider "anthropic" {
  api_key = var.anthropic_api_key
}

resource "anthropic_managed_environment" "python" {
  name        = "python-research"
  description = "sandbox for research agents"

  metadata = {
    team = "platform"
  }

  config = {
    type = "cloud"
    networking = {
      type                   = "limited"
      allow_mcp_servers      = true
      allow_package_managers = true
      allowed_hosts          = ["api.github.com", "pypi.org", "files.pythonhosted.org"]
    }
    packages = {
      type = "packages"
      pip  = ["pandas==2.3.0", "requests==2.32.3"]
      npm  = ["typescript@5.8.3"]
    }
  }
}

resource "anthropic_managed_agent" "repo_coder" {
  name        = "repo-coder"
  description = "analyzes and edits repositories"
  model_id    = "claude-sonnet-4-6"
  model_speed = "standard"
  system      = file("${path.module}/system.md")

  metadata = {
    owner = "platform-eng"
  }

  mcp_servers = [
    {
      type = "url"
      name = "github"
      url  = "https://mcp.example.com/github"
    }
  ]

  skills = [
    {
      type     = "anthropic"
      skill_id = "xlsx"
      version  = "latest"
    }
  ]

  tools = [
    {
      type = "agent_toolset_20260401"
      default_config = {
        enabled           = true
        permission_policy = "always_ask"
      }
      configs = [
        {
          name              = "read"
          enabled           = true
          permission_policy = "always_allow"
        },
        {
          name              = "write"
          enabled           = true
          permission_policy = "always_ask"
        }
      ]
    },
    {
      type            = "mcp_toolset"
      mcp_server_name = "github"
      default_config = {
        enabled           = true
        permission_policy = "always_ask"
      }
    }
  ]
}

resource "anthropic_managed_vault" "github" {
  display_name = "GitHub credentials"
}

resource "anthropic_managed_vault_credential" "github_oauth" {
  vault_id     = anthropic_managed_vault.github.id
  display_name = "GitHub MCP OAuth"

  auth = {
    type           = "mcp_oauth"
    mcp_server_url = "https://mcp.example.com/github"
    access_token   = var.github_access_token
  }
}

variable "anthropic_api_key" {
  type      = string
  sensitive = true
}

variable "github_access_token" {
  type      = string
  sensitive = true
}
