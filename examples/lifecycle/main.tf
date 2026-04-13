terraform {
  required_providers {
    anthropic = {
      source  = "anthropics-contrib/anthropic"
      version = "0.1.0"
    }
  }
}

# archive_on_destroy = true (default): resources are archived, not deleted.
# Set to false to hard-delete environments, vaults, and credentials.
# Note: agents ALWAYS archive regardless of this setting.
provider "anthropic" {
  archive_on_destroy = true
}

# ---------------------------------------------------------------
# 1. Create → Update flow
#
# Run `terraform apply` once to create, then change the values
# below and run `terraform apply` again to see updates in action.
# ---------------------------------------------------------------

# Environment: change networking from unrestricted to limited.
# Try changing `networking_type` from "unrestricted" to "limited".
resource "anthropic_managed_environment" "demo" {
  name        = "lifecycle-demo"
  description = var.env_description

  config = {
    type = "cloud"
    networking = {
      type                   = var.networking_type
      allow_mcp_servers      = var.networking_type == "limited" ? true : false
      allow_package_managers = var.networking_type == "limited" ? true : false
    }
  }
}

# Agent: update description, model speed, or system prompt.
# The `version` field increments automatically on each update,
# providing optimistic concurrency control.
resource "anthropic_managed_agent" "demo" {
  name        = "lifecycle-demo-agent"
  model_id    = var.model_id
  model_speed = var.model_speed
  description = var.agent_description
  system      = var.system_prompt

  metadata = {
    managed_by = "terraform"
    updated_at = timestamp()
  }
}

# Vault: rename by changing display_name.
resource "anthropic_managed_vault" "demo" {
  display_name = var.vault_name
}

# Credential: changing vault_id forces replacement (new credential).
# Changing auth fields updates in place.
resource "anthropic_managed_vault_credential" "demo" {
  vault_id     = anthropic_managed_vault.demo.id
  display_name = "demo-credential"

  auth = {
    type           = "mcp_oauth"
    mcp_server_url = var.mcp_server_url
    access_token   = var.access_token
  }
}

# ---------------------------------------------------------------
# 2. Outputs showing live state
# ---------------------------------------------------------------

output "environment_id" {
  value = anthropic_managed_environment.demo.id
}

output "agent_id" {
  value = anthropic_managed_agent.demo.id
}

output "agent_version" {
  description = "Increments on each update. Used for optimistic concurrency."
  value       = anthropic_managed_agent.demo.version
}

output "vault_id" {
  value = anthropic_managed_vault.demo.id
}

output "credential_id" {
  value = anthropic_managed_vault_credential.demo.id
}

output "credential_type" {
  value = anthropic_managed_vault_credential.demo.credential_type
}

# ---------------------------------------------------------------
# 3. Variables — change these to trigger updates
# ---------------------------------------------------------------

variable "networking_type" {
  type        = string
  default     = "unrestricted"
  description = "Change to 'limited' to restrict networking and see the update."

  validation {
    condition     = contains(["unrestricted", "limited"], var.networking_type)
    error_message = "Must be 'unrestricted' or 'limited'."
  }
}

variable "env_description" {
  type    = string
  default = "Demo environment for lifecycle examples"
}

variable "model_id" {
  type    = string
  default = "claude-sonnet-4-6"
}

variable "model_speed" {
  type    = string
  default = "standard"

  validation {
    condition     = contains(["standard", "fast"], var.model_speed)
    error_message = "Must be 'standard' or 'fast'."
  }
}

variable "agent_description" {
  type    = string
  default = "Initial description — change me to see an update"
}

variable "system_prompt" {
  type    = string
  default = "You are a helpful assistant."
}

variable "vault_name" {
  type    = string
  default = "lifecycle-demo-vault"
}

variable "mcp_server_url" {
  type    = string
  default = "https://mcp.example.com/demo"
}

variable "access_token" {
  type      = string
  sensitive = true
  default   = "demo-token-change-me"
}
