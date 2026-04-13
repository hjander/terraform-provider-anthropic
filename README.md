# Terraform Provider for Anthropic Managed Agents

Terraform provider for the [Anthropic Claude Managed Agents](https://platform.claude.com/docs/en/managed-agents/overview) API, built on the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework).

All resource schemas use **strongly-typed nested attributes** — no raw JSON strings except `input_schema` on custom tools (arbitrary JSON Schema).

## Quick start

```bash
# Prerequisites: Go 1.25+, Terraform 1.x
export ANTHROPIC_API_KEY="sk-ant-..."

# Build
go mod tidy
go build ./...

# Unit tests (no API key needed)
go test ./... -run "^Test[^AI]" -v

# API integration tests (live API, creates real resources and cleans up)
go test ./internal/provider/ -run "TestIntegration" -v -timeout 300s
```

## Repository layout

```
.
├── main.go                                  # Provider server entry point
├── go.mod
├── examples/
│   ├── minimal/main.tf                      # Simplest environment + agent
│   ├── basic/main.tf                        # Full-stack with MCP, skills, tools, vault
│   ├── basic/system.md                      # Example system prompt
│   ├── agent-tools/main.tf                  # All tool types: built-in, custom, MCP
│   ├── secure-environment/main.tf           # Locked-down networking + packages
│   ├── vault-credentials/main.tf            # Vault with multiple OAuth credentials
│   ├── multi-agent/main.tf                  # for_each pattern
│   └── import/README.md                     # Import guide for all resource types
└── internal/provider/
    ├── provider.go                           # Provider schema + Configure
    ├── client.go                             # HTTP client (GET/POST/DELETE)
    ├── helpers.go                            # TF ↔ Go type conversion helpers
    ├── resource_agent.go                     # anthropic_managed_agent
    ├── resource_environment.go               # anthropic_managed_environment
    ├── resource_vault.go                     # anthropic_managed_vault
    ├── resource_vault_credential.go          # anthropic_managed_vault_credential
    ├── datasource_agent.go                   # data.anthropic_managed_agent
    ├── datasource_environment.go             # data.anthropic_managed_environment
    ├── datasource_vault.go                   # data.anthropic_managed_vault
    ├── datasource_vault_credential.go        # data.anthropic_managed_vault_credential
    ├── acceptance_test.go                    # Live API integration tests
    ├── terraform_acc_test.go                 # Terraform acceptance tests (plugin-testing)
    ├── client_test.go                        # HTTP client unit tests
    ├── helpers_test.go                       # Type helper unit tests
    ├── provider_test.go                      # Provider schema/metadata tests
    ├── resource_agent_test.go                # Agent expand/flatten unit tests
    ├── resource_environment_test.go          # Environment expand/flatten tests
    ├── resource_vault_test.go                # Vault tests
    └── resource_vault_credential_test.go     # Credential expand/flatten tests
```

## Architecture

### Provider (`provider.go`)

The provider configures a shared HTTP `Client` and `archiveOnDestroy` flag, passed to all resources via `providerData`.

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `api_key` | string, sensitive, optional | `$ANTHROPIC_API_KEY` | Anthropic API key |
| `base_url` | string, optional | `https://api.anthropic.com` | API base URL |
| `anthropic_version` | string, optional | `2023-06-01` | `anthropic-version` header |
| `managed_agents_beta` | string, optional | `managed-agents-2026-04-01` | `anthropic-beta` header |
| `archive_on_destroy` | bool, optional | `true` | Archive instead of hard-delete |

### HTTP Client (`client.go`)

Thin wrapper around `net/http` with three methods: `Get`, `Post`, `Delete`. Every request sets three headers:

- `x-api-key` — API authentication
- `anthropic-version` — API version negotiation
- `anthropic-beta` — beta feature gate

Error responses are parsed into `apiErrorEnvelope` for structured error messages including `request_id`.

### Type helpers (`helpers.go`)

Bidirectional converters between Terraform Plugin Framework types and Go types:

| Function | Direction | Purpose |
|----------|-----------|---------|
| `mapFromTF` / `mapToTF` | TF ↔ `map[string]string` | Metadata maps |
| `setFromTF` / `sliceToSetTF` | TF ↔ `[]string` | Vault IDs, allowed hosts |
| `listValueStrings` / `listFromStrings` | TF ↔ `[]string` | Package lists |
| `parseJSONOrNull` | JSON string → Go | Custom tool `input_schema` |
| `mustJSON` | Go → JSON string | Flatten `input_schema` back |
| `stringOrNull` | string → `types.String` | Empty string → null |
| `anyString`, `anyBool`, `anyFloatAsInt64`, `anySliceToStrings` | `any` → typed | API response coercion |

---

## Resources

### `anthropic_managed_environment`

Cloud container environments with networking and package configuration.

**API endpoints:**
- Create: `POST /v1/environments`
- Read: `GET /v1/environments/{id}`
- Update: `POST /v1/environments/{id}`
- Delete: `DELETE /v1/environments/{id}` or `POST /v1/environments/{id}/archive`

**Schema:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | computed | Environment ID (`env_...`) |
| `name` | string | **yes** | Display name |
| `description` | string | optional | |
| `metadata` | map(string) | optional | Up to 16 key-value pairs |
| `archived` | bool | computed | Whether archived |
| `config` | object | **yes** | Cloud config (see below) |

**`config` nested object:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | **yes** | Must be `"cloud"` |
| `networking` | object | **yes** | See below |
| `packages` | object | optional | See below |

**`config.networking`:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | **yes** | `"unrestricted"` or `"limited"` |
| `allow_mcp_servers` | bool | optional | Allow MCP server connections |
| `allow_package_managers` | bool | optional | Allow package installations |
| `allowed_hosts` | set(string) | optional | Allowlisted hostnames |

**`config.packages`:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | **yes** | Must be `"packages"` |
| `apt` | list(string) | optional | APT packages |
| `cargo` | list(string) | optional | Cargo crates |
| `gem` | list(string) | optional | Ruby gems |
| `go` | list(string) | optional | Go modules |
| `npm` | list(string) | optional | NPM packages |
| `pip` | list(string) | optional | Python packages |

**Implementation notes:**
- `expandEnvironmentPayload` converts TF model → API payload via nested `basetypes.ObjectAsOptions` extraction.
- `flattenEnvironmentState` → `environmentConfigObjectFromAPI` reconstructs the three-level nested `types.Object` from the API's `map[string]any`.
- Delete branches on `archiveOnDestroy`: archive via POST or hard-delete via DELETE.

---

### `anthropic_managed_agent`

Agent definitions with model, system prompt, MCP servers, skills, and tools.

**API endpoints:**
- Create: `POST /v1/agents`
- Read: `GET /v1/agents/{id}`
- Update: `POST /v1/agents/{id}` (requires `version` for optimistic concurrency)
- Delete: `POST /v1/agents/{id}/archive` (archive only, no hard delete)

**Schema:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | computed | Agent ID (`agent_...`) |
| `name` | string | **yes** | 1–256 chars |
| `description` | string | optional | Up to 2048 chars |
| `model_id` | string | **yes** | e.g. `claude-sonnet-4-6` |
| `model_speed` | string | optional | `"standard"` or `"fast"` |
| `system` | string | optional | System prompt (up to 100k chars) |
| `metadata` | map(string) | optional | Up to 16 key-value pairs |
| `version` | int64 | computed | Server-managed, increments on update |
| `archived` | bool | computed | |
| `mcp_servers` | list(object) | optional | Max 20 (see below) |
| `skills` | list(object) | optional | Max 20 (see below) |
| `tools` | list(object) | optional | Max 128 across toolsets (see below) |

**`mcp_servers[]`:**

| Attribute | Type | Required |
|-----------|------|----------|
| `name` | string | **yes** |
| `type` | string | **yes** — must be `"url"` |
| `url` | string | **yes** |

**`skills[]`:**

| Attribute | Type | Required |
|-----------|------|----------|
| `type` | string | **yes** — `"anthropic"` or `"custom"` |
| `skill_id` | string | **yes** |
| `version` | string | optional |

**`tools[]`:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | **yes** | `"agent_toolset_20260401"`, `"mcp_toolset"`, or `"custom"` |
| `name` | string | optional | Required for custom tools |
| `description` | string | optional | For custom tools |
| `input_schema` | string (JSON) | optional | JSON Schema for custom tool input |
| `mcp_server_name` | string | optional | For `mcp_toolset` type |
| `default_config` | object | optional | Default tool config (see below) |
| `configs` | list(object) | optional | Per-tool overrides (see below) |

**`tools[].default_config` and `tools[].configs[]`:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | optional / **yes** for configs | Tool name to configure |
| `enabled` | bool | optional | |
| `permission_policy` | string | optional | `"always_allow"` or `"always_ask"` |

**`permission_policy` wire format:** The API expects `{"type": "always_allow"}` but the TF schema uses a flat string `"always_allow"`. The provider wraps/unwraps this automatically in `expandToolConfig` and `flattenToolConfigObj`.

**Implementation notes:**
- `buildAgentPayload(plan, includeVersion)` — when `includeVersion=true` (updates), adds the `version` field for optimistic concurrency.
- On Update, the provider first GETs the current agent to read the latest `version`, then POSTs with that version. This prevents stale-version conflicts.
- `model` can be a plain string (`"claude-sonnet-4-6"`) or an object (`{"id": "claude-sonnet-4-6", "speed": "fast"}`). `flattenModel` handles both forms.
- Delete always archives (agents cannot be hard-deleted per the API).

---

### `anthropic_managed_vault`

Credential vaults that group related credentials.

**API endpoints:**
- Create: `POST /v1/vaults`
- Read: `GET /v1/vaults/{id}`
- Update: `POST /v1/vaults/{id}`
- Delete: `DELETE /v1/vaults/{id}` or `POST /v1/vaults/{id}/archive`

**Schema:**

| Attribute | Type | Required |
|-----------|------|----------|
| `id` | string | computed |
| `display_name` | string | **yes** |
| `metadata` | map(string) | optional |
| `archived` | bool | computed |

---

### `anthropic_managed_vault_credential`

Credentials stored in vaults, currently supporting MCP OAuth.

**API endpoints:**
- Create: `POST /v1/vaults/{vault_id}/credentials`
- Read: `GET /v1/vaults/{vault_id}/credentials/{id}`
- Update: `POST /v1/vaults/{vault_id}/credentials/{id}`
- Delete: `DELETE /v1/vaults/{vault_id}/credentials/{id}` or archive

**Schema:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string | computed | |
| `vault_id` | string | **yes** | Parent vault |
| `display_name` | string | optional | |
| `metadata` | map(string) | optional | |
| `credential_type` | string | computed | Resolved from `auth.type` |
| `archived` | bool | computed | |
| `auth` | object, sensitive | **yes** | See below |

**`auth` nested object:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | **yes** | e.g. `"mcp_oauth"` |
| `mcp_server_url` | string | optional | MCP server URL |
| `access_token` | string, sensitive | optional | OAuth access token |
| `refresh` | object, sensitive | optional | See below |

**`auth.refresh`:**

| Attribute | Type | Required |
|-----------|------|----------|
| `client_id` | string | optional |
| `refresh_token` | string, sensitive | optional |
| `token_endpoint` | string | optional |
| `token_endpoint_auth` | string | optional |
| `scope` | string | optional |

**Implementation notes:**
- `flattenCredentialState` preserves `auth` from prior state on Read because the API does not echo back secrets.
- `credential_type` is extracted from `api.Auth["type"]` on Read.

---

## Data Sources

Each resource has a corresponding read-only data source for looking up existing resources by ID.

### `data.anthropic_managed_environment`

Look up an environment by ID. Exposes the same attributes as the resource (all computed).

```hcl
data "anthropic_managed_environment" "existing" {
  id = "env_01ABC..."
}
```

### `data.anthropic_managed_agent`

Look up an agent by ID. Exposes all agent attributes including tools, skills, and MCP servers.

```hcl
data "anthropic_managed_agent" "existing" {
  id = "agent_01ABC..."
}
```

### `data.anthropic_managed_vault`

Look up a vault by ID.

```hcl
data "anthropic_managed_vault" "existing" {
  id = "vlt_01ABC..."
}
```

### `data.anthropic_managed_vault_credential`

Look up a vault credential by vault ID and credential ID. Sensitive auth fields are not returned.

```hcl
data "anthropic_managed_vault_credential" "existing" {
  id       = "cred_01ABC..."
  vault_id = "vlt_01ABC..."
}
```

---

## Update semantics

| Resource | Update method | Special handling |
|----------|--------------|-----------------|
| Environment | `POST /v1/environments/{id}` | Full replace of config |
| Agent | `POST /v1/agents/{id}` | Requires `version` for optimistic concurrency; provider auto-fetches current version before update |
| Vault | `POST /v1/vaults/{id}` | Simple field update |
| Vault Credential | `POST /v1/vaults/{vault_id}/credentials/{id}` | Full replace of auth |

---

## Delete / archive semantics

| Resource | `archive_on_destroy = true` | `archive_on_destroy = false` |
|----------|----------------------------|------------------------------|
| Environment | `POST .../archive` | `DELETE /v1/environments/{id}` |
| Agent | `POST .../archive` | `POST .../archive` (always archives) |
| Vault | `POST .../archive` | `DELETE /v1/vaults/{id}` |
| Vault Credential | `POST .../archive` | `DELETE .../credentials/{id}` |

---

## Import

All resources support `terraform import` via ID passthrough:

```bash
terraform import anthropic_managed_environment.example env_01ABC...
terraform import anthropic_managed_agent.example agent_01ABC...
terraform import anthropic_managed_vault.example vlt_01ABC...
terraform import anthropic_managed_vault_credential.example vault_id/credential_id
```

**Vault credentials** use a composite import ID: `vault_id/credential_id` (e.g., `vlt_01ABC.../cred_01ABC...`).

---

## Tests

### Unit tests (no API key needed)

```bash
go test ./internal/provider/ -run "^Test[^AI]" -v
```

| File | What it covers |
|------|----------------|
| `client_test.go` | GET/POST/DELETE, API error parsing, empty body |
| `helpers_test.go` | All type conversion helpers, round-trips |
| `provider_test.go` | Schema attributes, resource/data source registration, New() |
| `resource_agent_test.go` | Payload build (minimal/versioned/speed/MCP/skills/tools), flatten (minimal/object model/MCP/tools/archived), model format, tool config expand with `permission_policy` wrapping |
| `resource_environment_test.go` | Expand/flatten (minimal/packages/archived/nil config), attr types |
| `resource_vault_test.go` | Flatten state, schema, metadata, configure lifecycle |
| `resource_vault_credential_test.go` | Auth expand (minimal/with refresh/null), payload build, flatten state, attr types |

### API integration tests (requires `ANTHROPIC_API_KEY`)

```bash
ANTHROPIC_API_KEY=sk-ant-... go test ./internal/provider/ -run "TestIntegration" -v -timeout 300s
```

| Test | Resources created | What it verifies |
|------|-------------------|-----------------|
| `TestIntegrationEnvironment_CRUD` | 1 environment | Create → Read → Update (networking change) → Delete |
| `TestIntegrationEnvironment_WithPackages` | 1 environment | Create with pip packages + limited networking |
| `TestIntegrationAgent_CRUD` | 1 agent | Create → Read → Update (version bump) → Archive → Verify `archived_at` |
| `TestIntegrationAgent_WithTools` | 1 agent | Create with `agent_toolset_20260401`, bash+read configs |
| `TestIntegrationAgent_WithSkills` | 1 agent | Create with `xlsx` anthropic skill |
| `TestIntegrationVault_CRUD` | 1 vault | Create → Read → Update → Delete |
| `TestIntegrationFullStack` | 1 env + 1 agent + 1 vault | Full lifecycle: create all → update agent (version bump) → cleanup |
| `TestIntegrationAgent_FlattenRoundtrip` | 1 agent | Create with model object + tools + skills + metadata → flatten to TF state → verify all fields round-trip correctly |

All integration tests clean up their resources via `defer`.

### Terraform acceptance tests (requires `ANTHROPIC_API_KEY`)

```bash
ANTHROPIC_API_KEY=sk-ant-... go test ./internal/provider/ -run "TestAcc" -v -timeout 300s
```

These use `terraform-plugin-testing` and go through the full Terraform plan/apply/destroy lifecycle via `ProtoV6ProviderFactories`.

| Test | What it verifies |
|------|-----------------|
| `TestAccEnvironment_basic` | Create → ImportState → Update (networking change) |
| `TestAccAgent_basic` | Create → ImportState → Update (add description) |
| `TestAccAgent_withTools` | Create with `agent_toolset_20260401`, tool configs |
| `TestAccVault_basic` | Create → ImportState → Update (rename) |

---

## Known design decisions

1. **`permission_policy` envelope**: The API uses `{"type": "always_allow"}` but the TF schema exposes a flat string. `expandToolConfig` wraps it; `flattenToolConfigObj` unwraps it. This keeps HCL clean.

2. **Secrets preserved from prior state**: `auth` on credentials is preserved from prior TF state during Read, because the API does not echo back sensitive fields like tokens.

3. **Agent model dual format**: The API accepts `model` as either a string or `{id, speed}` object. The provider sends a string when `model_speed` is null, an object when set. `flattenModel` handles both forms on read.

4. **`input_schema` as JSON string**: Custom tool `input_schema` is the one remaining JSON string field, because it represents an arbitrary JSON Schema object that would be impractical to model with fixed Terraform attributes.

5. **No session resource**: Sessions are ephemeral runtime objects whose status mutates server-side (`running` → `idle` → `terminated`). This is a poor fit for Terraform's declarative convergence model — sessions can't be "converged" back to a running state after termination. Use the API directly or a CI/CD wrapper for session lifecycle management.

6. **Optimistic concurrency on agents**: The API requires a `version` field on agent updates. The provider automatically GETs the current version before POSTing an update, so users never need to manage it.

7. **`archive_on_destroy` behavior**: When `true` (default), `terraform destroy` archives resources instead of hard-deleting them. Agents always archive regardless of this setting (the API does not support hard delete for agents).

8. **Composite vault credential import**: Vault credentials require `vault_id/credential_id` because the API path needs both (`/v1/vaults/{vault_id}/credentials/{id}`). A simple ID passthrough would produce a broken API path.

---

## Examples

| Directory | What it demonstrates |
|-----------|---------------------|
| [`examples/minimal/`](examples/minimal/) | Simplest possible setup: environment + agent |
| [`examples/basic/`](examples/basic/) | Full-stack: environment, agent with MCP + skills + tools, vault, credential |
| [`examples/agent-tools/`](examples/agent-tools/) | Agent with built-in toolset, custom tools, MCP toolset, and skills |
| [`examples/secure-environment/`](examples/secure-environment/) | Locked-down environment with limited networking, allowlisted hosts, and pre-installed packages |
| [`examples/vault-credentials/`](examples/vault-credentials/) | Vault with multiple OAuth credentials (with and without refresh tokens) |
| [`examples/multi-agent/`](examples/multi-agent/) | `for_each` pattern: multiple agents from a map |
| [`examples/import/`](examples/import/) | Import guide for all resource types including composite IDs |

### Quick: minimal setup

```hcl
resource "anthropic_managed_environment" "sandbox" {
  name = "sandbox"
  config {
    type = "cloud"
    networking { type = "unrestricted" }
  }
}

resource "anthropic_managed_agent" "assistant" {
  name     = "assistant"
  model_id = "claude-sonnet-4-6"
}
```

### Quick: agent with custom tool

```hcl
resource "anthropic_managed_agent" "deployer" {
  name     = "deployer"
  model_id = "claude-sonnet-4-6"

  tools {
    type        = "custom"
    name        = "deploy"
    description = "Deploy to staging or production"
    input_schema = jsonencode({
      type = "object"
      properties = {
        environment = { type = "string", enum = ["staging", "production"] }
        version     = { type = "string" }
      }
      required = ["environment", "version"]
    })
  }
}
```

### Quick: look up existing resources

```hcl
data "anthropic_managed_agent" "existing" {
  id = "agent_01ABC..."
}

data "anthropic_managed_environment" "existing" {
  id = "env_01ABC..."
}

output "agent_name" {
  value = data.anthropic_managed_agent.existing.name
}
```
