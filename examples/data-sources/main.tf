terraform {
  required_providers {
    anthropic = {
      source  = "anthropics-contrib/anthropic"
      version = "0.1.0"
    }
  }
}

provider "anthropic" {}

# ---------------------------------------------------------------
# Data sources let you read existing resources without managing
# them. Use them to reference infrastructure created elsewhere
# or to inspect live state.
# ---------------------------------------------------------------

# Look up an existing agent by ID.
data "anthropic_managed_agent" "existing" {
  id = var.agent_id
}

output "agent_name" {
  value = data.anthropic_managed_agent.existing.name
}

output "agent_model" {
  value = data.anthropic_managed_agent.existing.model_id
}

output "agent_version" {
  value = data.anthropic_managed_agent.existing.version
}

output "agent_archived" {
  value = data.anthropic_managed_agent.existing.archived
}

# Look up an existing environment by ID.
data "anthropic_managed_environment" "existing" {
  id = var.environment_id
}

output "environment_name" {
  value = data.anthropic_managed_environment.existing.name
}

output "environment_networking_type" {
  value = data.anthropic_managed_environment.existing.config.networking.type
}

# Look up an existing vault by ID.
data "anthropic_managed_vault" "existing" {
  id = var.vault_id
}

output "vault_display_name" {
  value = data.anthropic_managed_vault.existing.display_name
}

# Look up a credential by vault ID and credential ID.
# Note: sensitive auth fields are NOT returned by data sources.
data "anthropic_managed_vault_credential" "existing" {
  id       = var.credential_id
  vault_id = var.vault_id
}

output "credential_type" {
  value = data.anthropic_managed_vault_credential.existing.credential_type
}

# ---------------------------------------------------------------
# Combining data sources with resources: reference an existing
# environment when creating a new agent.
# ---------------------------------------------------------------

resource "anthropic_managed_agent" "new_agent" {
  name        = "agent-using-existing-env"
  model_id    = "claude-sonnet-4-6"
  description = "Agent referencing environment ${data.anthropic_managed_environment.existing.name}"

  metadata = {
    environment_id = data.anthropic_managed_environment.existing.id
  }
}

# ---------------------------------------------------------------
# Variables
# ---------------------------------------------------------------

variable "agent_id" {
  type        = string
  description = "ID of an existing agent to look up."
}

variable "environment_id" {
  type        = string
  description = "ID of an existing environment to look up."
}

variable "vault_id" {
  type        = string
  description = "ID of an existing vault to look up."
}

variable "credential_id" {
  type        = string
  description = "ID of an existing credential to look up."
}
