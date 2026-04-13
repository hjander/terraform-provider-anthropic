terraform {
  required_providers {
    anthropic = {
      source  = "anthropics-contrib/anthropic"
      version = "0.1.0"
    }
  }
}

provider "anthropic" {}

locals {
  agents = {
    reviewer = {
      name   = "code-reviewer"
      desc   = "Reviews PRs for correctness, style, and security issues"
      system = "You are a meticulous code reviewer. Focus on bugs, security, and maintainability."
    }
    writer = {
      name   = "code-writer"
      desc   = "Implements features from specifications"
      system = "You are a senior engineer. Write clean, well-tested code following project conventions."
    }
    documenter = {
      name   = "doc-writer"
      desc   = "Generates and maintains technical documentation"
      system = "You are a technical writer. Produce clear, accurate documentation from code."
    }
  }
}

resource "anthropic_managed_environment" "shared" {
  name = "shared-dev"

  config = {
    type = "cloud"
    networking = {
      type              = "limited"
      allow_mcp_servers = true
      allowed_hosts     = ["api.github.com"]
    }
  }
}

resource "anthropic_managed_agent" "team" {
  for_each = local.agents

  name        = each.value.name
  description = each.value.desc
  model_id    = "claude-sonnet-4-6"
  system      = each.value.system

  tools = [{
    type = "agent_toolset_20260401"
    default_config = {
      enabled           = true
      permission_policy = "always_allow"
    }
  }]
}

output "agent_ids" {
  value = { for k, v in anthropic_managed_agent.team : k => v.id }
}
