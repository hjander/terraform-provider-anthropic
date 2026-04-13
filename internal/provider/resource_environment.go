package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type environmentResource struct {
	client           *Client
	archiveOnDestroy bool
}

type environmentResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Metadata    types.Map    `tfsdk:"metadata"`
	Config      types.Object `tfsdk:"config"`
	Archived    types.Bool   `tfsdk:"archived"`
}

type environmentConfigModel struct {
	Type       types.String `tfsdk:"type"`
	Networking types.Object `tfsdk:"networking"`
	Packages   types.Object `tfsdk:"packages"`
}

type environmentNetworkingModel struct {
	Type                 types.String `tfsdk:"type"`
	AllowMCPServers      types.Bool   `tfsdk:"allow_mcp_servers"`
	AllowPackageManagers types.Bool   `tfsdk:"allow_package_managers"`
	AllowedHosts         types.Set    `tfsdk:"allowed_hosts"`
}

type environmentPackagesModel struct {
	Type  types.String `tfsdk:"type"`
	APT   types.List   `tfsdk:"apt"`
	Cargo types.List   `tfsdk:"cargo"`
	Gem   types.List   `tfsdk:"gem"`
	Go    types.List   `tfsdk:"go"`
	NPM   types.List   `tfsdk:"npm"`
	PIP   types.List   `tfsdk:"pip"`
}

type environmentAPIModel struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Config      map[string]any    `json:"config,omitempty"`
	ArchivedAt  *string           `json:"archived_at,omitempty"`
}

var _ resource.Resource = (*environmentResource)(nil)
var _ resource.ResourceWithImportState = (*environmentResource)(nil)

func NewEnvironmentResource() resource.Resource {
	return &environmentResource{}
}

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_environment"
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Managed environment for Anthropic Managed Agents.",
		Attributes: map[string]resourceschema.Attribute{
			"id":          resourceschema.StringAttribute{Computed: true},
			"name":        resourceschema.StringAttribute{Required: true},
			"description": resourceschema.StringAttribute{Optional: true},
			"metadata":    resourceschema.MapAttribute{Optional: true, ElementType: types.StringType},
			"archived":    resourceschema.BoolAttribute{Computed: true},
			"config": resourceschema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]resourceschema.Attribute{
					"type": resourceschema.StringAttribute{
						Required:   true,
						Validators: []validator.String{stringvalidator.OneOf("cloud")},
					},
					"networking": resourceschema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]resourceschema.Attribute{
							"type": resourceschema.StringAttribute{
								Required:   true,
								Validators: []validator.String{stringvalidator.OneOf("unrestricted", "limited")},
							},
						"allow_mcp_servers":      resourceschema.BoolAttribute{Optional: true, Computed: true},
						"allow_package_managers": resourceschema.BoolAttribute{Optional: true, Computed: true},
						"allowed_hosts":          resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
						},
					},
					"packages": resourceschema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Attributes: map[string]resourceschema.Attribute{
							"type": resourceschema.StringAttribute{
								Required:   true,
								Validators: []validator.String{stringvalidator.OneOf("packages")},
							},
						"apt":   resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
						"cargo": resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
						"gem":   resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
						"go":    resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
						"npm":   resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
						"pip":   resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
						},
					},
				},
			},
		},
	}
}

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := expandEnvironmentPayload(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api environmentAPIModel
	if err := r.client.Post(ctx, "/v1/environments", payload, &api); err != nil {
		resp.Diagnostics.AddError("Create environment failed", err.Error())
		return
	}

	state, diags := flattenEnvironmentState(ctx, api)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api environmentAPIModel
	if err := r.client.Get(ctx, fmt.Sprintf("/v1/environments/%s", state.ID.ValueString()), &api); err != nil {
		var nfe *NotFoundError
		if errors.As(err, &nfe) {
			// Resource was deleted outside Terraform; remove from state.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read environment failed", err.Error())
		return
	}

	newState, diags := flattenEnvironmentState(ctx, api)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan environmentResourceModel
	var state environmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := expandEnvironmentPayload(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api environmentAPIModel
	if err := r.client.Post(ctx, fmt.Sprintf("/v1/environments/%s", state.ID.ValueString()), payload, &api); err != nil {
		resp.Diagnostics.AddError("Update environment failed", err.Error())
		return
	}

	newState, diags := flattenEnvironmentState(ctx, api)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var err error
	if r.archiveOnDestroy {
		err = r.client.Post(ctx, fmt.Sprintf("/v1/environments/%s/archive", state.ID.ValueString()), map[string]any{}, nil)
	} else {
		err = r.client.Delete(ctx, fmt.Sprintf("/v1/environments/%s", state.ID.ValueString()))
	}
	if err != nil {
		resp.Diagnostics.AddError("Delete environment failed", err.Error())
	}
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func expandEnvironmentPayload(ctx context.Context, plan environmentResourceModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	meta, d := mapFromTF(ctx, plan.Metadata)
	diags.Append(d...)

	var cfg environmentConfigModel
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		diags.Append(plan.Config.As(ctx, &cfg, basetypes.ObjectAsOptions{})...)
	}
	if diags.HasError() {
		return nil, diags
	}

	var net environmentNetworkingModel
	diags.Append(cfg.Networking.As(ctx, &net, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}
	allowedHosts, d := setFromTF(ctx, net.AllowedHosts)
	diags.Append(d...)

	netPayload := map[string]any{
		"type": net.Type.ValueString(),
	}
	if net.Type.ValueString() == "limited" {
		netPayload["allow_mcp_servers"] = !net.AllowMCPServers.IsNull() && net.AllowMCPServers.ValueBool()
		netPayload["allow_package_managers"] = !net.AllowPackageManagers.IsNull() && net.AllowPackageManagers.ValueBool()
		netPayload["allowed_hosts"] = allowedHosts
	}

	payload := map[string]any{
		"name": plan.Name.ValueString(),
		"config": map[string]any{
			"type":       cfg.Type.ValueString(),
			"networking": netPayload,
		},
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload["description"] = plan.Description.ValueString()
	}
	if len(meta) > 0 {
		payload["metadata"] = meta
	}

	if !cfg.Packages.IsNull() && !cfg.Packages.IsUnknown() {
		var pkgs environmentPackagesModel
		diags.Append(cfg.Packages.As(ctx, &pkgs, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		payload["config"].(map[string]any)["packages"] = map[string]any{
			"type":  pkgs.Type.ValueString(),
			"apt":   listValueStrings(ctx, pkgs.APT, &diags),
			"cargo": listValueStrings(ctx, pkgs.Cargo, &diags),
			"gem":   listValueStrings(ctx, pkgs.Gem, &diags),
			"go":    listValueStrings(ctx, pkgs.Go, &diags),
			"npm":   listValueStrings(ctx, pkgs.NPM, &diags),
			"pip":   listValueStrings(ctx, pkgs.PIP, &diags),
		}
	}

	return payload, diags
}

func flattenEnvironmentState(ctx context.Context, api environmentAPIModel) (environmentResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := environmentResourceModel{
		ID:          types.StringValue(api.ID),
		Name:        types.StringValue(api.Name),
		Description: stringOrNull(api.Description),
		Archived:    types.BoolValue(api.ArchivedAt != nil),
	}

	meta, d := mapToTF(ctx, api.Metadata)
	diags.Append(d...)
	state.Metadata = meta

	cfg, d := environmentConfigObjectFromAPI(ctx, api.Config)
	diags.Append(d...)
	state.Config = cfg
	return state, diags
}

func environmentConfigObjectFromAPI(ctx context.Context, cfg map[string]any) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cfg == nil {
		return types.ObjectNull(map[string]attr.Type{
			"type":       types.StringType,
			"networking": types.ObjectType{AttrTypes: environmentNetworkingAttrTypes()},
			"packages":   types.ObjectType{AttrTypes: environmentPackagesAttrTypes()},
		}), diags
	}

	netMap, _ := cfg["networking"].(map[string]any)
	pkgMap, _ := cfg["packages"].(map[string]any)
	allowedHosts, d := sliceToSetTF(ctx, anySliceToStrings(netMap["allowed_hosts"]))
	diags.Append(d...)

	netObj, d := types.ObjectValue(environmentNetworkingAttrTypes(), map[string]attr.Value{
		"type":                   stringOrNull(anyString(netMap["type"])),
		"allow_mcp_servers":      types.BoolValue(anyBool(netMap["allow_mcp_servers"])),
		"allow_package_managers": types.BoolValue(anyBool(netMap["allow_package_managers"])),
		"allowed_hosts":          allowedHosts,
	})
	diags.Append(d...)

	pkgObj := types.ObjectNull(environmentPackagesAttrTypes())
	if pkgMap != nil {
		var aptList, cargoList, gemList, goList, npmList, pipList types.List
		aptList, d = listFromStrings(anySliceToStrings(pkgMap["apt"]))
		diags.Append(d...)
		cargoList, d = listFromStrings(anySliceToStrings(pkgMap["cargo"]))
		diags.Append(d...)
		gemList, d = listFromStrings(anySliceToStrings(pkgMap["gem"]))
		diags.Append(d...)
		goList, d = listFromStrings(anySliceToStrings(pkgMap["go"]))
		diags.Append(d...)
		npmList, d = listFromStrings(anySliceToStrings(pkgMap["npm"]))
		diags.Append(d...)
		pipList, d = listFromStrings(anySliceToStrings(pkgMap["pip"]))
		diags.Append(d...)
		pkgObj, d = types.ObjectValue(environmentPackagesAttrTypes(), map[string]attr.Value{
			"type":  stringOrNull(anyString(pkgMap["type"])),
			"apt":   aptList,
			"cargo": cargoList,
			"gem":   gemList,
			"go":    goList,
			"npm":   npmList,
			"pip":   pipList,
		})
		diags.Append(d...)
	}

	obj, d := types.ObjectValue(map[string]attr.Type{
		"type":       types.StringType,
		"networking": types.ObjectType{AttrTypes: environmentNetworkingAttrTypes()},
		"packages":   types.ObjectType{AttrTypes: environmentPackagesAttrTypes()},
	}, map[string]attr.Value{
		"type":       stringOrNull(anyString(cfg["type"])),
		"networking": netObj,
		"packages":   pkgObj,
	})
	diags.Append(d...)
	return obj, diags
}

func environmentNetworkingAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":                   types.StringType,
		"allow_mcp_servers":      types.BoolType,
		"allow_package_managers": types.BoolType,
		"allowed_hosts":          types.SetType{ElemType: types.StringType},
	}
}

func environmentPackagesAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"apt":   types.ListType{ElemType: types.StringType},
		"cargo": types.ListType{ElemType: types.StringType},
		"gem":   types.ListType{ElemType: types.StringType},
		"go":    types.ListType{ElemType: types.StringType},
		"npm":   types.ListType{ElemType: types.StringType},
		"pip":   types.ListType{ElemType: types.StringType},
	}
}
