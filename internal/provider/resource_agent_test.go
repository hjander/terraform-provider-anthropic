package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBuildAgentPayload_Minimal(t *testing.T) {
	plan := agentResourceModel{
		Name:       types.StringValue("test-agent"),
		ModelID:    types.StringValue("claude-sonnet-4-6"),
		ModelSpeed: types.StringNull(),
		Description: types.StringNull(),
		System:     types.StringNull(),
		Metadata:   types.MapNull(types.StringType),
		MCPServers: types.ListNull(types.ObjectType{AttrTypes: mcpServerAttrTypes()}),
		Skills:     types.ListNull(types.ObjectType{AttrTypes: skillAttrTypes()}),
		Tools:      types.ListNull(types.ObjectType{AttrTypes: toolAttrTypes()}),
		Version:    types.Int64Value(0),
	}

	payload, diags := buildAgentPayload(context.Background(), plan, false)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if payload["name"] != "test-agent" {
		t.Errorf("name=%v", payload["name"])
	}
	if payload["model"] != "claude-sonnet-4-6" {
		t.Errorf("model=%v", payload["model"])
	}
	if _, ok := payload["version"]; ok {
		t.Error("version should not be set")
	}
	if _, ok := payload["mcp_servers"]; ok {
		t.Error("mcp_servers should not be set for null input")
	}
}

func TestBuildAgentPayload_WithVersion(t *testing.T) {
	plan := agentResourceModel{
		Name:       types.StringValue("agent"),
		ModelID:    types.StringValue("claude-sonnet-4-6"),
		ModelSpeed: types.StringNull(),
		Description: types.StringNull(),
		System:     types.StringNull(),
		Metadata:   types.MapNull(types.StringType),
		MCPServers: types.ListNull(types.ObjectType{AttrTypes: mcpServerAttrTypes()}),
		Skills:     types.ListNull(types.ObjectType{AttrTypes: skillAttrTypes()}),
		Tools:      types.ListNull(types.ObjectType{AttrTypes: toolAttrTypes()}),
		Version:    types.Int64Value(5),
	}

	payload, diags := buildAgentPayload(context.Background(), plan, true)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if payload["version"] != int64(5) {
		t.Errorf("version=%v, want 5", payload["version"])
	}
}

func TestBuildAgentPayload_ModelSpeed(t *testing.T) {
	plan := agentResourceModel{
		Name:       types.StringValue("agent"),
		ModelID:    types.StringValue("claude-sonnet-4-6"),
		ModelSpeed: types.StringValue("fast"),
		Description: types.StringNull(),
		System:     types.StringNull(),
		Metadata:   types.MapNull(types.StringType),
		MCPServers: types.ListNull(types.ObjectType{AttrTypes: mcpServerAttrTypes()}),
		Skills:     types.ListNull(types.ObjectType{AttrTypes: skillAttrTypes()}),
		Tools:      types.ListNull(types.ObjectType{AttrTypes: toolAttrTypes()}),
		Version:    types.Int64Value(0),
	}

	payload, diags := buildAgentPayload(context.Background(), plan, false)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	model, ok := payload["model"].(map[string]any)
	if !ok {
		t.Fatalf("model should be map, got %T", payload["model"])
	}
	if model["id"] != "claude-sonnet-4-6" || model["speed"] != "fast" {
		t.Errorf("model=%v", model)
	}
}

func TestBuildAgentPayload_WithMCPServers(t *testing.T) {
	ctx := context.Background()
	mcpObjType := types.ObjectType{AttrTypes: mcpServerAttrTypes()}

	serverObj, d := types.ObjectValue(mcpServerAttrTypes(), map[string]attr.Value{
		"name": types.StringValue("github"),
		"type": types.StringValue("url"),
		"url":  types.StringValue("https://mcp.example.com/github"),
	})
	if d.HasError() {
		t.Fatal(d)
	}
	serverList, d := types.ListValue(mcpObjType, []attr.Value{serverObj})
	if d.HasError() {
		t.Fatal(d)
	}

	plan := agentResourceModel{
		Name:       types.StringValue("agent"),
		ModelID:    types.StringValue("claude-sonnet-4-6"),
		ModelSpeed: types.StringNull(),
		Description: types.StringNull(),
		System:     types.StringNull(),
		Metadata:   types.MapNull(types.StringType),
		MCPServers: serverList,
		Skills:     types.ListNull(types.ObjectType{AttrTypes: skillAttrTypes()}),
		Tools:      types.ListNull(types.ObjectType{AttrTypes: toolAttrTypes()}),
		Version:    types.Int64Value(0),
	}

	payload, diags := buildAgentPayload(ctx, plan, false)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	servers, ok := payload["mcp_servers"].([]map[string]any)
	if !ok {
		t.Fatalf("mcp_servers should be []map, got %T", payload["mcp_servers"])
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0]["name"] != "github" || servers[0]["url"] != "https://mcp.example.com/github" {
		t.Errorf("server=%v", servers[0])
	}
}

func TestBuildAgentPayload_WithSkills(t *testing.T) {
	ctx := context.Background()
	skillObjType := types.ObjectType{AttrTypes: skillAttrTypes()}

	skillObj, d := types.ObjectValue(skillAttrTypes(), map[string]attr.Value{
		"type":     types.StringValue("anthropic"),
		"skill_id": types.StringValue("xlsx"),
		"version":  types.StringNull(),
	})
	if d.HasError() {
		t.Fatal(d)
	}
	skillList, d := types.ListValue(skillObjType, []attr.Value{skillObj})
	if d.HasError() {
		t.Fatal(d)
	}

	plan := agentResourceModel{
		Name:       types.StringValue("agent"),
		ModelID:    types.StringValue("claude-sonnet-4-6"),
		ModelSpeed: types.StringNull(),
		Description: types.StringNull(),
		System:     types.StringNull(),
		Metadata:   types.MapNull(types.StringType),
		MCPServers: types.ListNull(types.ObjectType{AttrTypes: mcpServerAttrTypes()}),
		Skills:     skillList,
		Tools:      types.ListNull(types.ObjectType{AttrTypes: toolAttrTypes()}),
		Version:    types.Int64Value(0),
	}

	payload, diags := buildAgentPayload(ctx, plan, false)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	skills, ok := payload["skills"].([]map[string]any)
	if !ok {
		t.Fatalf("skills should be []map, got %T", payload["skills"])
	}
	if skills[0]["type"] != "anthropic" || skills[0]["skill_id"] != "xlsx" {
		t.Errorf("skill=%v", skills[0])
	}
	if _, hasVer := skills[0]["version"]; hasVer {
		t.Error("version should be omitted when null")
	}
}

func TestBuildAgentPayload_WithTools(t *testing.T) {
	ctx := context.Background()
	toolObjType := types.ObjectType{AttrTypes: toolAttrTypes()}

	dcObj, d := types.ObjectValue(toolConfigAttrTypes(), map[string]attr.Value{
		"name":              types.StringNull(),
		"enabled":           types.BoolValue(true),
		"permission_policy": types.StringValue("always_ask"),
	})
	if d.HasError() {
		t.Fatal(d)
	}

	cfgObj, d := types.ObjectValue(toolConfigAttrTypes(), map[string]attr.Value{
		"name":              types.StringValue("read"),
		"enabled":           types.BoolValue(true),
		"permission_policy": types.StringValue("always_allow"),
	})
	if d.HasError() {
		t.Fatal(d)
	}
	cfgList, d := types.ListValue(types.ObjectType{AttrTypes: toolConfigAttrTypes()}, []attr.Value{cfgObj})
	if d.HasError() {
		t.Fatal(d)
	}

	toolObj, d := types.ObjectValue(toolAttrTypes(), map[string]attr.Value{
		"type":            types.StringValue("agent_toolset_20260401"),
		"name":            types.StringNull(),
		"description":     types.StringNull(),
		"input_schema":    types.StringNull(),
		"mcp_server_name": types.StringNull(),
		"default_config":  dcObj,
		"configs":         cfgList,
	})
	if d.HasError() {
		t.Fatal(d)
	}
	toolList, d := types.ListValue(toolObjType, []attr.Value{toolObj})
	if d.HasError() {
		t.Fatal(d)
	}

	plan := agentResourceModel{
		Name:       types.StringValue("agent"),
		ModelID:    types.StringValue("claude-sonnet-4-6"),
		ModelSpeed: types.StringNull(),
		Description: types.StringNull(),
		System:     types.StringNull(),
		Metadata:   types.MapNull(types.StringType),
		MCPServers: types.ListNull(types.ObjectType{AttrTypes: mcpServerAttrTypes()}),
		Skills:     types.ListNull(types.ObjectType{AttrTypes: skillAttrTypes()}),
		Tools:      toolList,
		Version:    types.Int64Value(0),
	}

	payload, diags := buildAgentPayload(ctx, plan, false)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	tools, ok := payload["tools"].([]map[string]any)
	if !ok {
		t.Fatalf("tools should be []map, got %T", payload["tools"])
	}
	if tools[0]["type"] != "agent_toolset_20260401" {
		t.Errorf("tool type=%v", tools[0]["type"])
	}
	dc, ok := tools[0]["default_config"].(map[string]any)
	if !ok {
		t.Fatalf("default_config should be map, got %T", tools[0]["default_config"])
	}
	if dc["enabled"] != true {
		t.Errorf("default_config.enabled=%v", dc["enabled"])
	}
	dcPP, ok := dc["permission_policy"].(map[string]any)
	if !ok {
		t.Fatalf("default_config.permission_policy should be map, got %T", dc["permission_policy"])
	}
	if dcPP["type"] != "always_ask" {
		t.Errorf("default_config.permission_policy.type=%v", dcPP["type"])
	}
	configs, ok := tools[0]["configs"].([]map[string]any)
	if !ok || len(configs) != 1 {
		t.Fatalf("configs=%v", tools[0]["configs"])
	}
	if configs[0]["name"] != "read" {
		t.Errorf("config name=%v", configs[0]["name"])
	}
}

func TestFlattenAgentState_Minimal(t *testing.T) {
	api := agentAPIModel{
		ID:      "agent_123",
		Name:    "my-agent",
		Model:   "claude-sonnet-4-6",
		Version: 1,
	}

	state, diags := flattenAgentState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if state.ID.ValueString() != "agent_123" {
		t.Errorf("id=%s", state.ID.ValueString())
	}
	if state.ModelID.ValueString() != "claude-sonnet-4-6" {
		t.Errorf("model_id=%s", state.ModelID.ValueString())
	}
	if !state.ModelSpeed.IsNull() {
		t.Error("model_speed should be null for simple string model")
	}
	if !state.MCPServers.IsNull() {
		t.Error("mcp_servers should be null when empty")
	}
	if !state.Skills.IsNull() {
		t.Error("skills should be null when empty")
	}
	if !state.Tools.IsNull() {
		t.Error("tools should be null when empty")
	}
}

func TestFlattenAgentState_ObjectModel(t *testing.T) {
	api := agentAPIModel{
		ID:      "agent_123",
		Name:    "agent",
		Model:   map[string]any{"id": "claude-sonnet-4-6", "speed": "fast"},
		Version: 2,
	}

	state, diags := flattenAgentState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if state.ModelID.ValueString() != "claude-sonnet-4-6" {
		t.Errorf("model_id=%s", state.ModelID.ValueString())
	}
	if state.ModelSpeed.ValueString() != "fast" {
		t.Errorf("model_speed=%s", state.ModelSpeed.ValueString())
	}
}

func TestFlattenAgentState_WithMCPServers(t *testing.T) {
	api := agentAPIModel{
		ID:    "agent_1",
		Name:  "agent",
		Model: "claude-sonnet-4-6",
		MCPServers: []map[string]any{
			{"name": "github", "type": "url", "url": "https://mcp.example.com"},
		},
		Version: 1,
	}

	state, diags := flattenAgentState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if state.MCPServers.IsNull() {
		t.Fatal("mcp_servers should not be null")
	}
	if len(state.MCPServers.Elements()) != 1 {
		t.Errorf("expected 1 mcp server, got %d", len(state.MCPServers.Elements()))
	}
}

func TestFlattenAgentState_WithTools(t *testing.T) {
	api := agentAPIModel{
		ID:    "agent_1",
		Name:  "agent",
		Model: "claude-sonnet-4-6",
		Tools: []map[string]any{
			{
				"type": "agent_toolset_20260401",
				"default_config": map[string]any{
					"enabled":           true,
					"permission_policy": "always_ask",
				},
				"configs": []any{
					map[string]any{
						"name":              "read",
						"enabled":           true,
						"permission_policy": "always_allow",
					},
				},
			},
		},
		Version: 1,
	}

	state, diags := flattenAgentState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if state.Tools.IsNull() {
		t.Fatal("tools should not be null")
	}
	if len(state.Tools.Elements()) != 1 {
		t.Errorf("expected 1 tool, got %d", len(state.Tools.Elements()))
	}
}

func TestFlattenAgentState_Archived(t *testing.T) {
	ts := "2025-01-01T00:00:00Z"
	api := agentAPIModel{
		ID:         "agent_1",
		Name:       "agent",
		Model:      "claude-sonnet-4-6",
		ArchivedAt: &ts,
		Version:    1,
	}

	state, diags := flattenAgentState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !state.Archived.ValueBool() {
		t.Error("expected archived=true")
	}
}

func TestFlattenModel(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		wantID    string
		wantSpeed bool
	}{
		{"string", "claude-sonnet-4-6", "claude-sonnet-4-6", false},
		{"object", map[string]any{"id": "claude-opus-4-6", "speed": "fast"}, "claude-opus-4-6", true},
		{"nil", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, speed := flattenModel(tt.input)
			if tt.wantID != "" && id.ValueString() != tt.wantID {
				t.Errorf("id=%s, want %s", id.ValueString(), tt.wantID)
			}
			if tt.wantSpeed && speed.IsNull() {
				t.Error("expected speed to be set")
			}
			if !tt.wantSpeed && !speed.IsNull() {
				t.Error("expected speed to be null")
			}
		})
	}
}

func TestExpandToolConfig(t *testing.T) {
	c := toolConfigModel{
		Name:             types.StringValue("bash"),
		Enabled:          types.BoolValue(true),
		PermissionPolicy: types.StringValue("always_allow"),
	}
	m := expandToolConfig(c)
	if m["name"] != "bash" || m["enabled"] != true {
		t.Errorf("expandToolConfig=%v", m)
	}
	pp, ok := m["permission_policy"].(map[string]any)
	if !ok {
		t.Fatalf("permission_policy should be map, got %T", m["permission_policy"])
	}
	if pp["type"] != "always_allow" {
		t.Errorf("permission_policy.type=%v", pp["type"])
	}
}

func TestExpandToolConfig_Nulls(t *testing.T) {
	c := toolConfigModel{
		Name:             types.StringNull(),
		Enabled:          types.BoolNull(),
		PermissionPolicy: types.StringNull(),
	}
	m := expandToolConfig(c)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}
