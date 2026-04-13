# Lifecycle Example

Demonstrates create, update, and destroy behavior for all resource types.

## Usage

```bash
# 1. Create all resources
terraform apply

# 2. Update: change networking from unrestricted to limited
terraform apply -var='networking_type=limited'

# 3. Update: change agent description and speed
terraform apply -var='agent_description=Updated via Terraform' -var='model_speed=fast'

# 4. Update: rename the vault
terraform apply -var='vault_name=renamed-vault'

# 5. Destroy: archives all resources (default behavior)
terraform destroy
```

## Key Behaviors

- **Agent version**: increments automatically on each update (optimistic concurrency)
- **Vault credential vault_id**: changing this forces a new credential (replace, not update)
- **archive_on_destroy**: when `true` (default), `terraform destroy` archives resources instead of hard-deleting them. Agents always archive regardless of this setting.
- **Networking type change**: switching from `unrestricted` to `limited` adds fields like `allow_mcp_servers` and `allow_package_managers` to the config
