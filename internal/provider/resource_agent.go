package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
			"id": resourceschema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":        resourceschema.StringAttribute{Required: true, Description: "Display name for the agent."},
			"description": resourceschema.StringAttribute{Optional: true, Description: "Human-readable description of the agent's purpose."},
			"model_id":    resourceschema.StringAttribute{Required: true, Description: "Model identifier, e.g. claude-sonnet-4-6."},
			"model_speed": resourceschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Model speed: standard or fast.",
				Validators:  []validator.String{stringvalidator.OneOf("standard", "fast")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"system": resourceschema.StringAttribute{
				Optional:    true,
				Description: "System prompt (up to 100,000 chars).",
				Validators:  []validator.String{stringvalidator.LengthAtMost(100000)},
			},
			"metadata":    resourceschema.MapAttribute{Optional: true, ElementType: types.StringType, Description: "Arbitrary key-value metadata."},
			"version": resourceschema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"archived": resourceschema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"mcp_servers": resourceschema.ListNestedAttribute{
				Optional:    true,
				Description: "MCP server configurations (max 20).",
				Validators:  []validator.List{listvalidator.SizeAtMost(20)},
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
				Validators:  []validator.List{listvalidator.SizeAtMost(20)},
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
				Validators:  []validator.List{listvalidator.SizeAtMost(128)},
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
	tflog.Debug(ctx, "creating agent", map[string]any{"name": plan.Name.ValueString()})
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
	tflog.Debug(ctx, "created agent", map[string]any{"id": api.ID})
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
	tflog.Debug(ctx, "reading agent", map[string]any{"id": state.ID.ValueString()})
	var api agentAPIModel
	if err := r.client.Get(ctx, fmt.Sprintf("/v1/agents/%s", state.ID.ValueString()), &api); err != nil {
		var nfe *NotFoundError
		if errors.As(err, &nfe) {
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

	// Use version from state for optimistic concurrency; the API rejects stale versions,
	// surfacing concurrent modifications as errors rather than silently overwriting them.
	plan.Version = state.Version

	tflog.Debug(ctx, "updating agent", map[string]any{"id": state.ID.ValueString()})
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
	tflog.Debug(ctx, "updated agent", map[string]any{"id": api.ID})
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
	tflog.Debug(ctx, "deleting agent", map[string]any{"id": state.ID.ValueString()})
	// Agents only support archival, not hard-delete via the API.
	err := r.client.Post(ctx, fmt.Sprintf("/v1/agents/%s/archive", state.ID.ValueString()), map[string]any{}, nil)
	if err != nil {
		var nfe *NotFoundError
		if errors.As(err, &nfe) {
			return // Already gone
		}
		resp.Diagnostics.AddError("Delete agent failed", err.Error())
	}
}

func (r *agentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func buildAgentPayload(ctx context.Context, plan agentResourceModel, includeVersion bool) (agentRequestPayload, diag.Diagnostics) {
	var diags diag.Diagnostics
	meta, d := mapFromTF(ctx, plan.Metadata)
	diags.Append(d...)

	payload := agentRequestPayload{
		Name:     plan.Name.ValueString(),
		Metadata: meta,
		Model:    agentModelField{ID: plan.ModelID.ValueString()},
	}
	if !plan.ModelSpeed.IsNull() && !plan.ModelSpeed.IsUnknown() && plan.ModelSpeed.ValueString() != "" {
		payload.Model.Speed = plan.ModelSpeed.ValueString()
	}
	if includeVersion {
		v := plan.Version.ValueInt64()
		payload.Version = &v
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}
	if !plan.System.IsNull() && !plan.System.IsUnknown() {
		payload.System = plan.System.ValueString()
	}

	if !plan.MCPServers.IsNull() && !plan.MCPServers.IsUnknown() {
		var servers []mcpServerModel
		diags.Append(plan.MCPServers.ElementsAs(ctx, &servers, false)...)
		for _, s := range servers {
			payload.MCPServers = append(payload.MCPServers, mcpServerAPI{
				Name: s.Name.ValueString(),
				Type: s.Type.ValueString(),
				URL:  s.URL.ValueString(),
			})
		}
	}

	if !plan.Skills.IsNull() && !plan.Skills.IsUnknown() {
		var skills []skillModel
		diags.Append(plan.Skills.ElementsAs(ctx, &skills, false)...)
		for _, s := range skills {
			sk := skillAPI{
				Type:    s.Type.ValueString(),
				SkillID: s.SkillID.ValueString(),
			}
			if !s.Version.IsNull() && !s.Version.IsUnknown() {
				sk.Version = s.Version.ValueString()
			}
			payload.Skills = append(payload.Skills, sk)
		}
	}

	if !plan.Tools.IsNull() && !plan.Tools.IsUnknown() {
		var tools []toolModel
		diags.Append(plan.Tools.ElementsAs(ctx, &tools, false)...)
		for _, t := range tools {
			tool := toolAPI{
				Type: t.Type.ValueString(),
			}
			if !t.Name.IsNull() && !t.Name.IsUnknown() {
				tool.Name = t.Name.ValueString()
			}
			if !t.Description.IsNull() && !t.Description.IsUnknown() {
				tool.Description = t.Description.ValueString()
			}
			if !t.InputSchema.IsNull() && !t.InputSchema.IsUnknown() && t.InputSchema.ValueString() != "" {
				tool.InputSchema = json.RawMessage(t.InputSchema.ValueString())
			}
			if !t.MCPServerName.IsNull() && !t.MCPServerName.IsUnknown() {
				tool.MCPServerName = t.MCPServerName.ValueString()
			}
			if !t.DefaultConfig.IsNull() && !t.DefaultConfig.IsUnknown() {
				var dc toolConfigModel
				diags.Append(t.DefaultConfig.As(ctx, &dc, basetypes.ObjectAsOptions{})...)
				cfg := expandToolConfig(dc)
				tool.DefaultConfig = &cfg
			}
			if !t.Configs.IsNull() && !t.Configs.IsUnknown() {
				var configs []toolConfigModel
				diags.Append(t.Configs.ElementsAs(ctx, &configs, false)...)
				for _, c := range configs {
					tool.Configs = append(tool.Configs, expandToolConfig(c))
				}
			}
			payload.Tools = append(payload.Tools, tool)
		}
	}
	return payload, diags
}

func expandToolConfig(c toolConfigModel) toolConfigAPI {
	cfg := toolConfigAPI{}
	if !c.Name.IsNull() && !c.Name.IsUnknown() {
		cfg.Name = c.Name.ValueString()
	}
	if !c.Enabled.IsNull() && !c.Enabled.IsUnknown() {
		v := c.Enabled.ValueBool()
		cfg.Enabled = &v
	}
	if !c.PermissionPolicy.IsNull() && !c.PermissionPolicy.IsUnknown() {
		cfg.PermissionPolicy = &permissionPolicyAPI{Type: c.PermissionPolicy.ValueString()}
	}
	return cfg
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
		ModelID:     types.StringValue(api.Model.ID),
		ModelSpeed:  stringOrNull(api.Model.Speed),
	}
	meta, d := mapToTF(ctx, api.Metadata)
	diags.Append(d...)
	state.Metadata = meta

	mcpElemType := types.ObjectType{AttrTypes: mcpServerAttrTypes()}
	if len(api.MCPServers) == 0 {
		state.MCPServers = types.ListNull(mcpElemType)
	} else {
		var vals []attr.Value
		for _, s := range api.MCPServers {
			obj, d := types.ObjectValue(mcpServerAttrTypes(), map[string]attr.Value{
				"name": stringOrNull(s.Name),
				"type": stringOrNull(s.Type),
				"url":  stringOrNull(s.URL),
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
				"type":     stringOrNull(s.Type),
				"skill_id": stringOrNull(s.SkillID),
				"version":  stringOrNull(s.Version),
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

func flattenTool(t toolAPI, diags *diag.Diagnostics) attr.Value {
	dcObj := types.ObjectNull(toolConfigAttrTypes())
	if t.DefaultConfig != nil {
		var d diag.Diagnostics
		dcObj, d = flattenToolConfigObj(*t.DefaultConfig)
		*diags = append(*diags, d...)
	}

	configsList := types.ListNull(types.ObjectType{AttrTypes: toolConfigAttrTypes()})
	if len(t.Configs) > 0 {
		var configVals []attr.Value
		for _, c := range t.Configs {
			obj, d := flattenToolConfigObj(c)
			*diags = append(*diags, d...)
			configVals = append(configVals, obj)
		}
		if len(configVals) > 0 {
			var d diag.Diagnostics
			configsList, d = types.ListValue(types.ObjectType{AttrTypes: toolConfigAttrTypes()}, configVals)
			*diags = append(*diags, d...)
		}
	}

	inputSchema := types.StringNull()
	if len(t.InputSchema) > 0 {
		inputSchema = types.StringValue(string(t.InputSchema))
	}

	obj, d := types.ObjectValue(toolAttrTypes(), map[string]attr.Value{
		"type":            stringOrNull(t.Type),
		"name":            stringOrNull(t.Name),
		"description":     stringOrNull(t.Description),
		"input_schema":    inputSchema,
		"mcp_server_name": stringOrNull(t.MCPServerName),
		"default_config":  dcObj,
		"configs":         configsList,
	})
	*diags = append(*diags, d...)
	return obj
}

func flattenToolConfigObj(c toolConfigAPI) (types.Object, diag.Diagnostics) {
	enabled := types.BoolNull()
	if c.Enabled != nil {
		enabled = types.BoolValue(*c.Enabled)
	}
	pp := types.StringNull()
	if c.PermissionPolicy != nil {
		pp = stringOrNull(c.PermissionPolicy.Type)
	}
	return types.ObjectValue(toolConfigAttrTypes(), map[string]attr.Value{
		"name":              stringOrNull(c.Name),
		"enabled":           enabled,
		"permission_policy": pp,
	})
}
