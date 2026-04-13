package provider

import (
	"context"
	"encoding/json"
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
	if payload.Name != "test-agent" {
		t.Errorf("name=%v", payload.Name)
	}
	if payload.Model.ID != "claude-sonnet-4-6" {
		t.Errorf("model.id=%v", payload.Model.ID)
	}
	if payload.Model.Speed != "" {
		t.Errorf("model.speed should be empty, got %v", payload.Model.Speed)
	}
	if payload.Version != nil {
		t.Error("version should not be set")
	}
	if len(payload.MCPServers) != 0 {
		t.Error("mcp_servers should be empty for null input")
	}

	// Verify the model serializes as a plain string when speed is empty.
	b, _ := json.Marshal(payload.Model)
	if string(b) != `"claude-sonnet-4-6"` {
		t.Errorf("model should serialize as string, got %s", string(b))
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
	if payload.Version == nil || *payload.Version != 5 {
		t.Errorf("version=%v, want 5", payload.Version)
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
	if payload.Model.ID != "claude-sonnet-4-6" || payload.Model.Speed != "fast" {
		t.Errorf("model=%+v", payload.Model)
	}

	// Verify the model serializes as an object when speed is set.
	b, _ := json.Marshal(payload.Model)
	if string(b) == `"claude-sonnet-4-6"` {
		t.Error("model should serialize as object when speed is set")
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
	if len(payload.MCPServers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(payload.MCPServers))
	}
	if payload.MCPServers[0].Name != "github" || payload.MCPServers[0].URL != "https://mcp.example.com/github" {
		t.Errorf("server=%+v", payload.MCPServers[0])
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
	if len(payload.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(payload.Skills))
	}
	if payload.Skills[0].Type != "anthropic" || payload.Skills[0].SkillID != "xlsx" {
		t.Errorf("skill=%+v", payload.Skills[0])
	}
	if payload.Skills[0].Version != "" {
		t.Error("version should be empty when null")
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
	if len(payload.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(payload.Tools))
	}
	if payload.Tools[0].Type != "agent_toolset_20260401" {
		t.Errorf("tool type=%v", payload.Tools[0].Type)
	}
	if payload.Tools[0].DefaultConfig == nil {
		t.Fatal("default_config should be set")
	}
	if payload.Tools[0].DefaultConfig.Enabled == nil || !*payload.Tools[0].DefaultConfig.Enabled {
		t.Errorf("default_config.enabled=%v", payload.Tools[0].DefaultConfig.Enabled)
	}
	if payload.Tools[0].DefaultConfig.PermissionPolicy == nil || payload.Tools[0].DefaultConfig.PermissionPolicy.Type != "always_ask" {
		t.Errorf("default_config.permission_policy=%+v", payload.Tools[0].DefaultConfig.PermissionPolicy)
	}
	if len(payload.Tools[0].Configs) != 1 {
		t.Fatalf("configs=%v", payload.Tools[0].Configs)
	}
	if payload.Tools[0].Configs[0].Name != "read" {
		t.Errorf("config name=%v", payload.Tools[0].Configs[0].Name)
	}
}

func TestFlattenAgentState_Minimal(t *testing.T) {
	api := agentAPIModel{
		ID:      "agent_123",
		Name:    "my-agent",
		Model:   agentModelField{ID: "claude-sonnet-4-6"},
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
		Model:   agentModelField{ID: "claude-sonnet-4-6", Speed: "fast"},
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
		Model: agentModelField{ID: "claude-sonnet-4-6"},
		MCPServers: []mcpServerAPI{
			{Name: "github", Type: "url", URL: "https://mcp.example.com"},
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
	enabled := true
	api := agentAPIModel{
		ID:    "agent_1",
		Name:  "agent",
		Model: agentModelField{ID: "claude-sonnet-4-6"},
		Tools: []toolAPI{
			{
				Type: "agent_toolset_20260401",
				DefaultConfig: &toolConfigAPI{
					Enabled:          &enabled,
					PermissionPolicy: &permissionPolicyAPI{Type: "always_ask"},
				},
				Configs: []toolConfigAPI{
					{
						Name:             "read",
						Enabled:          &enabled,
						PermissionPolicy: &permissionPolicyAPI{Type: "always_allow"},
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
		Model:      agentModelField{ID: "claude-sonnet-4-6"},
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

func TestAgentModelField_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantID    string
		wantSpeed string
	}{
		{"string", `"claude-sonnet-4-6"`, "claude-sonnet-4-6", ""},
		{"object", `{"id":"claude-opus-4-6","speed":"fast"}`, "claude-opus-4-6", "fast"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m agentModelField
			if err := json.Unmarshal([]byte(tt.input), &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if m.ID != tt.wantID {
				t.Errorf("id=%s, want %s", m.ID, tt.wantID)
			}
			if m.Speed != tt.wantSpeed {
				t.Errorf("speed=%s, want %s", m.Speed, tt.wantSpeed)
			}
		})
	}
}

func TestAgentModelField_MarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input agentModelField
		want  string
	}{
		{"string", agentModelField{ID: "claude-sonnet-4-6"}, `"claude-sonnet-4-6"`},
		{"object", agentModelField{ID: "claude-opus-4-6", Speed: "fast"}, `{"id":"claude-opus-4-6","speed":"fast"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(b) != tt.want {
				t.Errorf("got %s, want %s", string(b), tt.want)
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
	cfg := expandToolConfig(c)
	if cfg.Name != "bash" {
		t.Errorf("name=%v", cfg.Name)
	}
	if cfg.Enabled == nil || !*cfg.Enabled {
		t.Error("enabled should be true")
	}
	if cfg.PermissionPolicy == nil || cfg.PermissionPolicy.Type != "always_allow" {
		t.Errorf("permission_policy=%+v", cfg.PermissionPolicy)
	}
}

func TestExpandToolConfig_Nulls(t *testing.T) {
	c := toolConfigModel{
		Name:             types.StringNull(),
		Enabled:          types.BoolNull(),
		PermissionPolicy: types.StringNull(),
	}
	cfg := expandToolConfig(c)
	if cfg.Name != "" {
		t.Errorf("expected empty name, got %v", cfg.Name)
	}
	if cfg.Enabled != nil {
		t.Errorf("expected nil enabled, got %v", cfg.Enabled)
	}
	if cfg.PermissionPolicy != nil {
		t.Errorf("expected nil permission_policy, got %v", cfg.PermissionPolicy)
	}
}
