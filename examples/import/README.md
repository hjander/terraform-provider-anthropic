# Import Examples

All provider resources support `terraform import`. This directory shows how to
import existing API resources into Terraform state.

## Simple imports (by resource ID)

```bash
# Environment
terraform import anthropic_managed_environment.example env_01JSHGK...

# Agent
terraform import anthropic_managed_agent.example agent_01JSHGK...

# Vault
terraform import anthropic_managed_vault.example vlt_01JSHGK...
```

## Vault credential import (composite ID)

Vault credentials require both vault ID and credential ID separated by `/`:

```hcl
resource "anthropic_managed_vault_credential" "imported" {
  vault_id = "vlt_01JSHGK..."

  auth = {
    type           = "mcp_oauth"
    mcp_server_url = "https://mcp.example.com/service"
    access_token   = var.access_token
  }
}
```

```bash
terraform import anthropic_managed_vault_credential.imported vlt_01JSHGK.../cred_01JSHGK...
```

After import, the `auth` block from your config will be used for subsequent
operations. The API does not return secrets on GET, so these values are preserved
from your HCL config and prior state.
