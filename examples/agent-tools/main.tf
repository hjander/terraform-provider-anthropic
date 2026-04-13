terraform {
  required_providers {
    anthropic = {
      source  = "anthropics-contrib/anthropic"
      version = "0.1.0"
    }
  }
}

provider "anthropic" {}

# Agent with the built-in toolset and custom tools.
resource "anthropic_managed_agent" "developer" {
  name        = "developer-agent"
  description = "Full-stack developer with file and bash access"
  model_id = "claude-sonnet-4-6"
  system      = <<-EOT
    You are a senior software engineer. Write clean, tested code.
    Always explain your reasoning before making changes.
  EOT

  metadata = {
    team    = "engineering"
    purpose = "code-review"
  }

  # Built-in toolset with per-tool overrides.
  tools = [{
    type = "agent_toolset_20260401"

    default_config = {
      enabled           = true
      permission_policy = "always_ask"
    }

    configs = [
      {
        name              = "bash"
        enabled           = true
        permission_policy = "always_allow"
      },
      {
        name              = "read"
        enabled           = true
        permission_policy = "always_allow"
      },
      {
        name              = "write"
        enabled           = true
        permission_policy = "always_ask"
      },
    ]
  },

  # Custom tool with JSON Schema input.
  {
    type        = "custom"
    name        = "deploy"
    description = "Deploy the application to staging or production"
    input_schema = jsonencode({
      type = "object"
      properties = {
        environment = {
          type = "string"
          enum = ["staging", "production"]
        }
        version = {
          type = "string"
        }
      }
      required = ["environment", "version"]
    })
  },

  # MCP-connected toolset.
  {
    type            = "mcp_toolset"
    mcp_server_name = "github-mcp"
    default_config = {
      enabled           = true
      permission_policy = "always_ask"
    }
  }]

  mcp_servers = [{
    name = "github-mcp"
    type = "url"
    url  = "https://mcp.example.com/github"
  }]

  # Anthropic skill.
  skills = [{
    type     = "anthropic"
    skill_id = "xlsx"
    version  = "latest"
  }]
}

output "agent_id" {
  value = anthropic_managed_agent.developer.id
}

output "agent_version" {
  value = anthropic_managed_agent.developer.version
}
