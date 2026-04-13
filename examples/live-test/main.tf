terraform {
  required_providers {
    anthropic = {
      source = "anthropics-contrib/anthropic"
    }
  }
}

provider "anthropic" {}

# ── Environments ──────────────────────────────────────────────

resource "anthropic_managed_environment" "sandbox" {
  name        = "live-test-sandbox"
  description = "Sandbox environment - now with limited networking"

  metadata = {
    test_run = "live-test"
    updated  = "true"
  }

  config = {
    type = "cloud"
    networking = {
      type                   = "limited"
      allow_package_managers = true
      allowed_hosts          = ["api.github.com"]
    }
  }
}

resource "anthropic_managed_environment" "production" {
  name        = "live-test-production"
  description = "Locked-down environment with limited networking"

  metadata = {
    test_run   = "live-test"
    compliance = "internal"
  }

  config = {
    type = "cloud"
    networking = {
      type                   = "limited"
      allow_package_managers = true
      allowed_hosts          = ["api.github.com", "pypi.org"]
    }
    packages = {
      type = "packages"
      pip  = ["requests==2.32.3"]
    }
  }
}

# ── Agents ────────────────────────────────────────────────────

resource "anthropic_managed_agent" "minimal" {
  name        = "live-test-minimal"
  description = "Updated: now has a description"
  model_id    = "claude-sonnet-4-6"
}

resource "anthropic_managed_agent" "tooled" {
  name        = "live-test-tooled"
  description = "Agent with built-in toolset and configs"
  model_id    = "claude-sonnet-4-6"
  system      = "You are a helpful coding assistant."

  tools = [
    {
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
        }
      ]
    }
  ]
}

resource "anthropic_managed_agent" "custom_tool" {
  name        = "live-test-custom-tool"
  description = "Agent with a custom tool"
  model_id    = "claude-sonnet-4-6"

  tools = [
    {
      type        = "custom"
      name        = "get_weather"
      description = "Returns current weather for a city"
      input_schema = jsonencode({
        type = "object"
        properties = {
          city = {
            type        = "string"
            description = "City name"
          }
        }
        required = ["city"]
      })
    }
  ]
}

# ── Vault ─────────────────────────────────────────────────────

resource "anthropic_managed_vault" "test" {
  display_name = "live-test-vault"

  metadata = {
    test_run = "live-test"
  }
}

# ── Outputs ───────────────────────────────────────────────────

output "sandbox_env_id" {
  value = anthropic_managed_environment.sandbox.id
}

output "production_env_id" {
  value = anthropic_managed_environment.production.id
}

output "minimal_agent_id" {
  value = anthropic_managed_agent.minimal.id
}

output "tooled_agent_id" {
  value = anthropic_managed_agent.tooled.id
}

output "custom_tool_agent_id" {
  value = anthropic_managed_agent.custom_tool.id
}

output "vault_id" {
  value = anthropic_managed_vault.test.id
}
