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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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
			"id": resourceschema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":        resourceschema.StringAttribute{Required: true},
			"description": resourceschema.StringAttribute{Optional: true},
			"metadata":    resourceschema.MapAttribute{Optional: true, ElementType: types.StringType},
			"archived": resourceschema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
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
						"allow_mcp_servers": resourceschema.BoolAttribute{
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
						},
						"allow_package_managers": resourceschema.BoolAttribute{
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
						},
						"allowed_hosts": resourceschema.SetAttribute{
							Optional:      true,
							Computed:      true,
							ElementType:   types.StringType,
							PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
						},
						},
					},
					"packages": resourceschema.SingleNestedAttribute{
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
						Attributes: map[string]resourceschema.Attribute{
							"type": resourceschema.StringAttribute{
								Required:   true,
								Validators: []validator.String{stringvalidator.OneOf("packages")},
							},
						"apt":   resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}},
						"cargo": resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}},
						"gem":   resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}},
						"go":    resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}},
						"npm":   resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}},
						"pip":   resourceschema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}},
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

func expandEnvironmentPayload(ctx context.Context, plan environmentResourceModel) (environmentRequestPayload, diag.Diagnostics) {
	var diags diag.Diagnostics
	meta, d := mapFromTF(ctx, plan.Metadata)
	diags.Append(d...)

	var cfg environmentConfigModel
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		diags.Append(plan.Config.As(ctx, &cfg, basetypes.ObjectAsOptions{})...)
	}
	if diags.HasError() {
		return environmentRequestPayload{}, diags
	}

	var net environmentNetworkingModel
	diags.Append(cfg.Networking.As(ctx, &net, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return environmentRequestPayload{}, diags
	}
	allowedHosts, d := setFromTF(ctx, net.AllowedHosts)
	diags.Append(d...)

	netAPI := environmentNetworkingAPI{
		Type: net.Type.ValueString(),
	}
	if net.Type.ValueString() == "limited" {
		netAPI.AllowMCPServers = !net.AllowMCPServers.IsNull() && net.AllowMCPServers.ValueBool()
		netAPI.AllowPackageManagers = !net.AllowPackageManagers.IsNull() && net.AllowPackageManagers.ValueBool()
		netAPI.AllowedHosts = allowedHosts
	}

	payload := environmentRequestPayload{
		Name:     plan.Name.ValueString(),
		Metadata: meta,
		Config: &environmentConfigAPI{
			Type:       cfg.Type.ValueString(),
			Networking: netAPI,
		},
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}

	if !cfg.Packages.IsNull() && !cfg.Packages.IsUnknown() {
		var pkgs environmentPackagesModel
		diags.Append(cfg.Packages.As(ctx, &pkgs, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return environmentRequestPayload{}, diags
		}
		payload.Config.Packages = &environmentPackagesAPI{
			Type:  pkgs.Type.ValueString(),
			APT:   listValueStrings(ctx, pkgs.APT, &diags),
			Cargo: listValueStrings(ctx, pkgs.Cargo, &diags),
			Gem:   listValueStrings(ctx, pkgs.Gem, &diags),
			Go:    listValueStrings(ctx, pkgs.Go, &diags),
			NPM:   listValueStrings(ctx, pkgs.NPM, &diags),
			PIP:   listValueStrings(ctx, pkgs.PIP, &diags),
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

func environmentConfigObjectFromAPI(ctx context.Context, cfg *environmentConfigAPI) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	configObjType := map[string]attr.Type{
		"type":       types.StringType,
		"networking": types.ObjectType{AttrTypes: environmentNetworkingAttrTypes()},
		"packages":   types.ObjectType{AttrTypes: environmentPackagesAttrTypes()},
	}
	if cfg == nil {
		return types.ObjectNull(configObjType), diags
	}

	allowedHosts, d := sliceToSetTF(ctx, cfg.Networking.AllowedHosts)
	diags.Append(d...)

	netObj, d := types.ObjectValue(environmentNetworkingAttrTypes(), map[string]attr.Value{
		"type":                   stringOrNull(cfg.Networking.Type),
		"allow_mcp_servers":      types.BoolValue(cfg.Networking.AllowMCPServers),
		"allow_package_managers": types.BoolValue(cfg.Networking.AllowPackageManagers),
		"allowed_hosts":          allowedHosts,
	})
	diags.Append(d...)

	pkgObj := types.ObjectNull(environmentPackagesAttrTypes())
	if cfg.Packages != nil {
		var aptList, cargoList, gemList, goList, npmList, pipList types.List
		aptList, d = listFromStrings(cfg.Packages.APT)
		diags.Append(d...)
		cargoList, d = listFromStrings(cfg.Packages.Cargo)
		diags.Append(d...)
		gemList, d = listFromStrings(cfg.Packages.Gem)
		diags.Append(d...)
		goList, d = listFromStrings(cfg.Packages.Go)
		diags.Append(d...)
		npmList, d = listFromStrings(cfg.Packages.NPM)
		diags.Append(d...)
		pipList, d = listFromStrings(cfg.Packages.PIP)
		diags.Append(d...)
		pkgObj, d = types.ObjectValue(environmentPackagesAttrTypes(), map[string]attr.Value{
			"type":  stringOrNull(cfg.Packages.Type),
			"apt":   aptList,
			"cargo": cargoList,
			"gem":   gemList,
			"go":    goList,
			"npm":   npmList,
			"pip":   pipList,
		})
		diags.Append(d...)
	}

	obj, d := types.ObjectValue(configObjType, map[string]attr.Value{
		"type":       stringOrNull(cfg.Type),
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
