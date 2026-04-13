package provider

import "encoding/json"

// ---------- Agent API types ----------

// agentModelField handles the polymorphic "model" field which can be either
// a plain string ("claude-sonnet-4-6") or an object ({"id": "...", "speed": "fast"}).
type agentModelField struct {
	ID    string `json:"id,omitempty"`
	Speed string `json:"speed,omitempty"`
}

func (m *agentModelField) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.ID = s
		m.Speed = ""
		return nil
	}
	type alias agentModelField
	var obj alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*m = agentModelField(obj)
	return nil
}

func (m agentModelField) MarshalJSON() ([]byte, error) {
	if m.Speed == "" {
		return json.Marshal(m.ID)
	}
	type alias agentModelField
	return json.Marshal(alias(m))
}

type permissionPolicyAPI struct {
	Type string `json:"type"`
}

type toolConfigAPI struct {
	Name             string               `json:"name,omitempty"`
	Enabled          *bool                `json:"enabled,omitempty"`
	PermissionPolicy *permissionPolicyAPI `json:"permission_policy,omitempty"`
}

type toolAPI struct {
	Type          string          `json:"type"`
	Name          string          `json:"name,omitempty"`
	Description   string          `json:"description,omitempty"`
	InputSchema   json.RawMessage `json:"input_schema,omitempty"`
	MCPServerName string          `json:"mcp_server_name,omitempty"`
	DefaultConfig *toolConfigAPI  `json:"default_config,omitempty"`
	Configs       []toolConfigAPI `json:"configs,omitempty"`
}

type mcpServerAPI struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

type skillAPI struct {
	Type    string `json:"type"`
	SkillID string `json:"skill_id"`
	Version string `json:"version,omitempty"`
}

type agentAPIModel struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	System      string            `json:"system,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Model       agentModelField   `json:"model,omitempty"`
	MCPServers  []mcpServerAPI    `json:"mcp_servers,omitempty"`
	Skills      []skillAPI        `json:"skills,omitempty"`
	Tools       []toolAPI         `json:"tools,omitempty"`
	Version     int64             `json:"version,omitempty"`
	ArchivedAt  *string           `json:"archived_at,omitempty"`
}

// agentRequestPayload is used for Create/Update requests where the model field
// may need to be either a string or object depending on whether speed is set.
type agentRequestPayload struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	System      string            `json:"system,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Model       agentModelField   `json:"model"`
	MCPServers  []mcpServerAPI    `json:"mcp_servers,omitempty"`
	Skills      []skillAPI        `json:"skills,omitempty"`
	Tools       []toolAPI         `json:"tools,omitempty"`
	Version     *int64            `json:"version,omitempty"`
}

// ---------- Environment API types ----------

type environmentNetworkingAPI struct {
	Type                 string   `json:"type"`
	AllowMCPServers      *bool    `json:"allow_mcp_servers,omitempty"`
	AllowPackageManagers *bool    `json:"allow_package_managers,omitempty"`
	AllowedHosts         []string `json:"allowed_hosts,omitempty"`
}

type environmentPackagesAPI struct {
	Type  string   `json:"type"`
	APT   []string `json:"apt,omitempty"`
	Cargo []string `json:"cargo,omitempty"`
	Gem   []string `json:"gem,omitempty"`
	Go    []string `json:"go,omitempty"`
	NPM   []string `json:"npm,omitempty"`
	PIP   []string `json:"pip,omitempty"`
}

type environmentConfigAPI struct {
	Type       string                    `json:"type"`
	Networking environmentNetworkingAPI  `json:"networking"`
	Packages   *environmentPackagesAPI   `json:"packages,omitempty"`
}

type environmentAPIModel struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Metadata    map[string]string     `json:"metadata,omitempty"`
	Config      *environmentConfigAPI `json:"config,omitempty"`
	ArchivedAt  *string               `json:"archived_at,omitempty"`
}

type environmentRequestPayload struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Metadata    map[string]string     `json:"metadata,omitempty"`
	Config      *environmentConfigAPI `json:"config,omitempty"`
}

// ---------- Vault API types ----------

type vaultAPIModel struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	ArchivedAt  *string           `json:"archived_at,omitempty"`
}

type vaultRequestPayload struct {
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ---------- Credential API types ----------

type credentialRefreshAPI struct {
	ClientID          string `json:"client_id,omitempty"`
	RefreshToken      string `json:"refresh_token,omitempty"`
	TokenEndpoint     string `json:"token_endpoint,omitempty"`
	TokenEndpointAuth string `json:"token_endpoint_auth,omitempty"`
	Scope             string `json:"scope,omitempty"`
}

type credentialAuthAPI struct {
	Type         string                `json:"type"`
	MCPServerURL string                `json:"mcp_server_url,omitempty"`
	AccessToken  string                `json:"access_token,omitempty"`
	Refresh      *credentialRefreshAPI `json:"refresh,omitempty"`
}

type credentialAPIModel struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Auth        credentialAuthAPI `json:"auth,omitempty"`
	ArchivedAt  *string           `json:"archived_at,omitempty"`
}

type credentialRequestPayload struct {
	DisplayName string            `json:"display_name,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Auth        credentialAuthAPI `json:"auth"`
}
