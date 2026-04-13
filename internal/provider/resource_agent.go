package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type agentResource struct {
	client           *Client
	archiveOnDestroy bool
}

type agentResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	ModelID    types.String `tfsdk:"model_id"`
	ModelSpeed types.String `tfsdk:"model_speed"`
	System     types.String `tfsdk:"system"`
	Metadata   types.Map    `tfsdk:"metadata"`
	MCPServers types.List   `tfsdk:"mcp_servers"`
	Skills     types.List   `tfsdk:"skills"`
	Tools      types.List   `tfsdk:"tools"`
	Version    types.Int64  `tfsdk:"version"`
	Archived   types.Bool   `tfsdk:"archived"`
}

type mcpServerModel struct {
	Name types.String `tfsdk:"name"`
	Type types.String `tfsdk:"type"`
	URL  types.String `tfsdk:"url"`
}

type skillModel struct {
	Type    types.String `tfsdk:"type"`
	SkillID types.String `tfsdk:"skill_id"`
	Version types.String `tfsdk:"version"`
}

type toolModel struct {
	Type          types.String `tfsdk:"type"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	InputSchema   types.String `tfsdk:"input_schema"`
	MCPServerName types.String `tfsdk:"mcp_server_name"`
	DefaultConfig types.Object `tfsdk:"default_config"`
	Configs       types.List   `tfsdk:"configs"`
}

type toolConfigModel struct {
	Name             types.String `tfsdk:"name"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	PermissionPolicy types.String `tfsdk:"permission_policy"`
}

type agentAPIModel struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	System      string            `json:"system,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Model       any               `json:"model,omitempty"`
	MCPServers  []map[string]any  `json:"mcp_servers,omitempty"`
	Skills      []map[string]any  `json:"skills,omitempty"`
	Tools       []map[string]any  `json:"tools,omitempty"`
	Version     int64             `json:"version,omitempty"`
	ArchivedAt  *string           `json:"archived_at,omitempty"`
}

var _ resource.Resource = (*agentResource)(nil)
var _ resource.ResourceWithImportState = (*agentResource)(nil)
var _ resource.ResourceWithConfigValidators = (*agentResource)(nil)

func NewAgentResource() resource.Resource { return &agentResource{} }

func (r *agentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_agent"
}

func (r *agentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *providerData, got %T", req.ProviderData),
		)
		return
	}
	r.client = pd.client
	r.archiveOnDestroy = pd.archiveOnDestroy
}

func toolConfigAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":              types.StringType,
		"enabled":           types.BoolType,
		"permission_policy": types.StringType,
	}
}

func mcpServerAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
		"type": types.StringType,
		"url":  types.StringType,
	}
}

func skillAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":     types.StringType,
		"skill_id": types.StringType,
		"version":  types.StringType,
	}
}

func toolAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":            types.StringType,
		"name":            types.StringType,
		"description":     types.StringType,
		"input_schema":    types.StringType,
		"mcp_server_name": types.StringType,
		"default_config":  types.ObjectType{AttrTypes: toolConfigAttrTypes()},
		"configs":         types.ListType{ElemType: types.ObjectType{AttrTypes: toolConfigAttrTypes()}},
	}
}

func (r *agentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Managed agent configuration.",
		Attributes: map[string]resourceschema.Attribute{
			"id":          resourceschema.StringAttribute{Computed: true},
			"name":        resourceschema.StringAttribute{Required: true},
			"description": resourceschema.StringAttribute{Optional: true},
			"model_id":    resourceschema.StringAttribute{Required: true, Description: "Model identifier, e.g. claude-sonnet-4-6."},
			"model_speed": resourceschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Model speed: standard or fast.",
				Validators:  []validator.String{stringvalidator.OneOf("standard", "fast")},
			},
			"system":      resourceschema.StringAttribute{Optional: true, Description: "System prompt (up to 100,000 chars)."},
			"metadata":    resourceschema.MapAttribute{Optional: true, ElementType: types.StringType},
			"version":     resourceschema.Int64Attribute{Computed: true},
			"archived":    resourceschema.BoolAttribute{Computed: true},
			"mcp_servers": resourceschema.ListNestedAttribute{
				Optional:    true,
				Description: "MCP server configurations (max 20).",
				NestedObject: resourceschema.NestedAttributeObject{
					Attributes: map[string]resourceschema.Attribute{
						"name": resourceschema.StringAttribute{Required: true, Description: "Unique MCP server name."},
						"type": resourceschema.StringAttribute{
							Required:    true,
							Description: "Must be 'url'.",
							Validators:  []validator.String{stringvalidator.OneOf("url")},
						},
						"url":  resourceschema.StringAttribute{Required: true, Description: "MCP server URL."},
					},
				},
			},
			"skills": resourceschema.ListNestedAttribute{
				Optional:    true,
				Description: "Skill configurations (max 20).",
				NestedObject: resourceschema.NestedAttributeObject{
					Attributes: map[string]resourceschema.Attribute{
						"type": resourceschema.StringAttribute{
							Required:    true,
							Description: "Skill type: anthropic or custom.",
							Validators:  []validator.String{stringvalidator.OneOf("anthropic", "custom")},
						},
						"skill_id": resourceschema.StringAttribute{Required: true, Description: "Skill identifier."},
						"version":  resourceschema.StringAttribute{Optional: true, Description: "Skill version."},
					},
				},
			},
			"tools": resourceschema.ListNestedAttribute{
				Optional:    true,
				Description: "Tool configurations (max 128 tools across all toolsets).",
				NestedObject: resourceschema.NestedAttributeObject{
					Attributes: map[string]resourceschema.Attribute{
						"type": resourceschema.StringAttribute{
							Required:    true,
							Description: "Tool type: agent_toolset_20260401, mcp_toolset, or custom.",
							Validators:  []validator.String{stringvalidator.OneOf("agent_toolset_20260401", "mcp_toolset", "custom")},
						},
						"name":            resourceschema.StringAttribute{Optional: true, Description: "Tool name (required for custom tools)."},
						"description":     resourceschema.StringAttribute{Optional: true, Description: "Tool description (for custom tools)."},
						"input_schema":    resourceschema.StringAttribute{Optional: true, Description: "JSON Schema for custom tool input."},
						"mcp_server_name": resourceschema.StringAttribute{Optional: true, Description: "MCP server name (for mcp_toolset type)."},
						"default_config": resourceschema.SingleNestedAttribute{
							Optional:    true,
							Description: "Default configuration for toolset tools.",
							Attributes: map[string]resourceschema.Attribute{
								"name":              resourceschema.StringAttribute{Optional: true},
								"enabled":           resourceschema.BoolAttribute{Optional: true},
								"permission_policy": resourceschema.StringAttribute{
									Optional:    true,
									Description: "always_allow or always_ask.",
									Validators:  []validator.String{stringvalidator.OneOf("always_allow", "always_ask")},
								},
							},
						},
						"configs": resourceschema.ListNestedAttribute{
							Optional:    true,
							Description: "Per-tool configuration overrides.",
							NestedObject: resourceschema.NestedAttributeObject{
								Attributes: map[string]resourceschema.Attribute{
									"name":              resourceschema.StringAttribute{Required: true, Description: "Tool name to configure."},
									"enabled":           resourceschema.BoolAttribute{Optional: true},
									"permission_policy": resourceschema.StringAttribute{
										Optional:    true,
										Description: "always_allow or always_ask.",
										Validators:  []validator.String{stringvalidator.OneOf("always_allow", "always_ask")},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Enforces tool-variant constraints at plan time; the API would reject these too, but plan-time errors give better UX.
func (r *agentResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&toolVariantValidator{},
	}
}

type toolVariantValidator struct{}

func (v *toolVariantValidator) Description(_ context.Context) string {
	return "Validates tool-variant field requirements."
}

func (v *toolVariantValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *toolVariantValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config agentResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.Tools.IsNull() || config.Tools.IsUnknown() {
		return
	}
	var tools []toolModel
	resp.Diagnostics.Append(config.Tools.ElementsAs(ctx, &tools, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for i, t := range tools {
		toolType := t.Type.ValueString()
		switch toolType {
		case "custom":
			if t.Name.IsNull() || t.Name.IsUnknown() || t.Name.ValueString() == "" {
				resp.Diagnostics.AddAttributeError(
					path.Root("tools").AtListIndex(i).AtName("name"),
					"Missing required field for custom tool",
					"Custom tools require a name.",
				)
			}
		case "mcp_toolset":
			if t.MCPServerName.IsNull() || t.MCPServerName.IsUnknown() || t.MCPServerName.ValueString() == "" {
				resp.Diagnostics.AddAttributeError(
					path.Root("tools").AtListIndex(i).AtName("mcp_server_name"),
					"Missing required field for mcp_toolset",
					"MCP toolset tools require mcp_server_name.",
				)
			}
		}
	}
}

func (r *agentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan agentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, diags := buildAgentPayload(ctx, plan, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api agentAPIModel
	if err := r.client.Post(ctx, "/v1/agents", payload, &api); err != nil {
		resp.Diagnostics.AddError("Create agent failed", err.Error())
		return
	}
	state, diags := flattenAgentState(ctx, api)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *agentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state agentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api agentAPIModel
	if err := r.client.Get(ctx, fmt.Sprintf("/v1/agents/%s", state.ID.ValueString()), &api); err != nil {
		var nfe *NotFoundError
		if errors.As(err, &nfe) {
			// Resource was deleted outside Terraform; remove from state.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read agent failed", err.Error())
		return
	}
	newState, diags := flattenAgentState(ctx, api)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *agentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan agentResourceModel
	var state agentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch current version for optimistic concurrency; the API rejects stale versions.
	var current agentAPIModel
	if err := r.client.Get(ctx, fmt.Sprintf("/v1/agents/%s", state.ID.ValueString()), &current); err != nil {
		resp.Diagnostics.AddError("Get agent before update failed", err.Error())
		return
	}
	plan.Version = types.Int64Value(current.Version)

	payload, diags := buildAgentPayload(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api agentAPIModel
	if err := r.client.Post(ctx, fmt.Sprintf("/v1/agents/%s", state.ID.ValueString()), payload, &api); err != nil {
		resp.Diagnostics.AddError("Update agent failed", err.Error())
		return
	}
	newState, diags := flattenAgentState(ctx, api)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *agentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state agentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Post(ctx, fmt.Sprintf("/v1/agents/%s/archive", state.ID.ValueString()), map[string]any{}, nil); err != nil {
		resp.Diagnostics.AddError("Archive agent failed", err.Error())
	}
}

func (r *agentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func buildAgentPayload(ctx context.Context, plan agentResourceModel, includeVersion bool) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	meta, d := mapFromTF(ctx, plan.Metadata)
	diags.Append(d...)

	payload := map[string]any{
		"name":  plan.Name.ValueString(),
		"model": plan.ModelID.ValueString(),
	}
	if !plan.ModelSpeed.IsNull() && !plan.ModelSpeed.IsUnknown() && plan.ModelSpeed.ValueString() != "" {
		payload["model"] = map[string]any{
			"id":    plan.ModelID.ValueString(),
			"speed": plan.ModelSpeed.ValueString(),
		}
	}
	if includeVersion {
		payload["version"] = plan.Version.ValueInt64()
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload["description"] = plan.Description.ValueString()
	}
	if !plan.System.IsNull() && !plan.System.IsUnknown() {
		payload["system"] = plan.System.ValueString()
	}
	if len(meta) > 0 {
		payload["metadata"] = meta
	}

	if !plan.MCPServers.IsNull() && !plan.MCPServers.IsUnknown() {
		var servers []mcpServerModel
		diags.Append(plan.MCPServers.ElementsAs(ctx, &servers, false)...)
		var apiServers []map[string]any
		for _, s := range servers {
			apiServers = append(apiServers, map[string]any{
				"name": s.Name.ValueString(),
				"type": s.Type.ValueString(),
				"url":  s.URL.ValueString(),
			})
		}
		if len(apiServers) > 0 {
			payload["mcp_servers"] = apiServers
		}
	}

	if !plan.Skills.IsNull() && !plan.Skills.IsUnknown() {
		var skills []skillModel
		diags.Append(plan.Skills.ElementsAs(ctx, &skills, false)...)
		var apiSkills []map[string]any
		for _, s := range skills {
			sk := map[string]any{
				"type":     s.Type.ValueString(),
				"skill_id": s.SkillID.ValueString(),
			}
			if !s.Version.IsNull() && !s.Version.IsUnknown() {
				sk["version"] = s.Version.ValueString()
			}
			apiSkills = append(apiSkills, sk)
		}
		if len(apiSkills) > 0 {
			payload["skills"] = apiSkills
		}
	}

	if !plan.Tools.IsNull() && !plan.Tools.IsUnknown() {
		var tools []toolModel
		diags.Append(plan.Tools.ElementsAs(ctx, &tools, false)...)
		var apiTools []map[string]any
		for _, t := range tools {
			tool := map[string]any{
				"type": t.Type.ValueString(),
			}
			if !t.Name.IsNull() && !t.Name.IsUnknown() {
				tool["name"] = t.Name.ValueString()
			}
			if !t.Description.IsNull() && !t.Description.IsUnknown() {
				tool["description"] = t.Description.ValueString()
			}
			if !t.InputSchema.IsNull() && !t.InputSchema.IsUnknown() {
				var schema map[string]any
				diags.Append(parseJSONOrNull(t.InputSchema, &schema)...)
				if schema != nil {
					tool["input_schema"] = schema
				}
			}
			if !t.MCPServerName.IsNull() && !t.MCPServerName.IsUnknown() {
				tool["mcp_server_name"] = t.MCPServerName.ValueString()
			}
			if !t.DefaultConfig.IsNull() && !t.DefaultConfig.IsUnknown() {
				var dc toolConfigModel
				diags.Append(t.DefaultConfig.As(ctx, &dc, basetypes.ObjectAsOptions{})...)
				tool["default_config"] = expandToolConfig(dc)
			}
			if !t.Configs.IsNull() && !t.Configs.IsUnknown() {
				var configs []toolConfigModel
				diags.Append(t.Configs.ElementsAs(ctx, &configs, false)...)
				var apiConfigs []map[string]any
				for _, c := range configs {
					apiConfigs = append(apiConfigs, expandToolConfig(c))
				}
				tool["configs"] = apiConfigs
			}
			apiTools = append(apiTools, tool)
		}
		if len(apiTools) > 0 {
			payload["tools"] = apiTools
		}
	}
	return payload, diags
}

func expandToolConfig(c toolConfigModel) map[string]any {
	m := map[string]any{}
	if !c.Name.IsNull() && !c.Name.IsUnknown() {
		m["name"] = c.Name.ValueString()
	}
	if !c.Enabled.IsNull() && !c.Enabled.IsUnknown() {
		m["enabled"] = c.Enabled.ValueBool()
	}
	if !c.PermissionPolicy.IsNull() && !c.PermissionPolicy.IsUnknown() {
		// API expects permission_policy as {"type": "always_allow"}; TF schema exposes it as a flat string for cleaner HCL.
		m["permission_policy"] = map[string]any{"type": c.PermissionPolicy.ValueString()}
	}
	return m
}

func flattenAgentState(ctx context.Context, api agentAPIModel) (agentResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := agentResourceModel{
		ID:          types.StringValue(api.ID),
		Name:        types.StringValue(api.Name),
		Description: stringOrNull(api.Description),
		System:      stringOrNull(api.System),
		Version:     types.Int64Value(api.Version),
		Archived:    types.BoolValue(api.ArchivedAt != nil),
	}
	meta, d := mapToTF(ctx, api.Metadata)
	diags.Append(d...)
	state.Metadata = meta
	state.ModelID, state.ModelSpeed = flattenModel(api.Model)

	mcpElemType := types.ObjectType{AttrTypes: mcpServerAttrTypes()}
	if len(api.MCPServers) == 0 {
		state.MCPServers = types.ListNull(mcpElemType)
	} else {
		var vals []attr.Value
		for _, s := range api.MCPServers {
			obj, d := types.ObjectValue(mcpServerAttrTypes(), map[string]attr.Value{
				"name": stringOrNull(anyString(s["name"])),
				"type": stringOrNull(anyString(s["type"])),
				"url":  stringOrNull(anyString(s["url"])),
			})
			diags.Append(d...)
			vals = append(vals, obj)
		}
		list, d := types.ListValue(mcpElemType, vals)
		diags.Append(d...)
		state.MCPServers = list
	}

	skillElemType := types.ObjectType{AttrTypes: skillAttrTypes()}
	if len(api.Skills) == 0 {
		state.Skills = types.ListNull(skillElemType)
	} else {
		var vals []attr.Value
		for _, s := range api.Skills {
			obj, d := types.ObjectValue(skillAttrTypes(), map[string]attr.Value{
				"type":     stringOrNull(anyString(s["type"])),
				"skill_id": stringOrNull(anyString(s["skill_id"])),
				"version":  stringOrNull(anyString(s["version"])),
			})
			diags.Append(d...)
			vals = append(vals, obj)
		}
		list, d := types.ListValue(skillElemType, vals)
		diags.Append(d...)
		state.Skills = list
	}

	toolElemType := types.ObjectType{AttrTypes: toolAttrTypes()}
	if len(api.Tools) == 0 {
		state.Tools = types.ListNull(toolElemType)
	} else {
		var vals []attr.Value
		for _, t := range api.Tools {
			vals = append(vals, flattenTool(t, &diags))
		}
		list, d := types.ListValue(toolElemType, vals)
		diags.Append(d...)
		state.Tools = list
	}

	return state, diags
}

func flattenTool(t map[string]any, diags *diag.Diagnostics) attr.Value {
	dcObj := types.ObjectNull(toolConfigAttrTypes())
	if dc, ok := t["default_config"].(map[string]any); ok {
		var d diag.Diagnostics
		dcObj, d = flattenToolConfigObj(dc)
		*diags = append(*diags, d...)
	}

	configListType := types.ListType{ElemType: types.ObjectType{AttrTypes: toolConfigAttrTypes()}}
	configsList := types.ListNull(types.ObjectType{AttrTypes: toolConfigAttrTypes()})
	if cfgs, ok := t["configs"].([]any); ok && len(cfgs) > 0 {
		var configVals []attr.Value
		for _, c := range cfgs {
			cm, _ := c.(map[string]any)
			if cm == nil {
				continue
			}
			obj, d := flattenToolConfigObj(cm)
			*diags = append(*diags, d...)
			configVals = append(configVals, obj)
		}
		if len(configVals) > 0 {
			var d diag.Diagnostics
			configsList, d = types.ListValue(configListType.ElemType, configVals)
			*diags = append(*diags, d...)
		}
	}

	inputSchema := types.StringNull()
	if is, ok := t["input_schema"]; ok && is != nil {
		var d diag.Diagnostics
		inputSchema, d = mustJSON(is)
		*diags = append(*diags, d...)
	}

	obj, d := types.ObjectValue(toolAttrTypes(), map[string]attr.Value{
		"type":            stringOrNull(anyString(t["type"])),
		"name":            stringOrNull(anyString(t["name"])),
		"description":     stringOrNull(anyString(t["description"])),
		"input_schema":    inputSchema,
		"mcp_server_name": stringOrNull(anyString(t["mcp_server_name"])),
		"default_config":  dcObj,
		"configs":         configsList,
	})
	*diags = append(*diags, d...)
	return obj
}

func flattenToolConfigObj(m map[string]any) (types.Object, diag.Diagnostics) {
	enabled := types.BoolNull()
	if v, ok := m["enabled"]; ok {
		enabled = types.BoolValue(anyBool(v))
	}
	// Unwrap the API's {"type": "..."} envelope back to a flat string for TF state.
	pp := types.StringNull()
	if ppObj, ok := m["permission_policy"].(map[string]any); ok {
		pp = stringOrNull(anyString(ppObj["type"]))
	} else if ppStr, ok := m["permission_policy"].(string); ok {
		pp = stringOrNull(ppStr)
	}
	return types.ObjectValue(toolConfigAttrTypes(), map[string]attr.Value{
		"name":              stringOrNull(anyString(m["name"])),
		"enabled":           enabled,
		"permission_policy": pp,
	})
}

func flattenModel(v any) (types.String, types.String) {
	s, ok := v.(string)
	if ok {
		return types.StringValue(s), types.StringNull()
	}
	m, ok := v.(map[string]any)
	if !ok {
		return types.StringNull(), types.StringNull()
	}
	return stringOrNull(anyString(m["id"])), stringOrNull(anyString(m["speed"]))
}
