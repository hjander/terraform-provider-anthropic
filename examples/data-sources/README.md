# Data Sources Example

Demonstrates reading existing resources via data sources without managing them.

## Usage

```bash
# Look up existing resources by their IDs
terraform apply \
  -var='agent_id=agent_01ABC...' \
  -var='environment_id=env_01DEF...' \
  -var='vault_id=vlt_01GHI...' \
  -var='credential_id=cred_01JKL...'
```

## When to Use Data Sources

- **Cross-team references**: read resources managed by another team's Terraform workspace
- **Inspection**: check the current state of resources (archived status, version, config)
- **Composition**: use a data source ID as input to a new resource (e.g., reference an existing environment when creating an agent)

## Key Behaviors

- Data sources are **read-only** — they never create, update, or delete resources
- Vault credential data sources do **not** return sensitive auth fields (access tokens, refresh tokens)
- If a resource is not found, the data source returns a clear error: `"Agent not found"` / `"Environment not found"` etc.
