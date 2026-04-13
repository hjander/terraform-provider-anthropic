package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type vaultCredentialResource struct {
	client           *Client
	archiveOnDestroy bool
}

type vaultCredentialResourceModel struct {
	ID             types.String `tfsdk:"id"`
	VaultID        types.String `tfsdk:"vault_id"`
	DisplayName    types.String `tfsdk:"display_name"`
	Metadata       types.Map    `tfsdk:"metadata"`
	Auth           types.Object `tfsdk:"auth"`
	Archived       types.Bool   `tfsdk:"archived"`
	CredentialType types.String `tfsdk:"credential_type"`
}

type credentialAuthModel struct {
	Type           types.String `tfsdk:"type"`
	MCPServerURL   types.String `tfsdk:"mcp_server_url"`
	AccessToken    types.String `tfsdk:"access_token"`
	RefreshConfig  types.Object `tfsdk:"refresh"`
}

type credentialRefreshModel struct {
	ClientID          types.String `tfsdk:"client_id"`
	RefreshToken      types.String `tfsdk:"refresh_token"`
	TokenEndpoint     types.String `tfsdk:"token_endpoint"`
	TokenEndpointAuth types.String `tfsdk:"token_endpoint_auth"`
	Scope             types.String `tfsdk:"scope"`
}

var _ resource.Resource = (*vaultCredentialResource)(nil)
var _ resource.ResourceWithImportState = (*vaultCredentialResource)(nil)

func NewVaultCredentialResource() resource.Resource { return &vaultCredentialResource{} }

func (r *vaultCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_vault_credential"
}

func (r *vaultCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func credentialRefreshAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"client_id":           types.StringType,
		"refresh_token":       types.StringType,
		"token_endpoint":      types.StringType,
		"token_endpoint_auth": types.StringType,
		"scope":               types.StringType,
	}
}

func credentialAuthAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":           types.StringType,
		"mcp_server_url": types.StringType,
		"access_token":   types.StringType,
		"refresh":        types.ObjectType{AttrTypes: credentialRefreshAttrTypes()},
	}
}

func (r *vaultCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Vault credential for MCP OAuth or other auth types.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vault_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Parent vault ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": resourceschema.StringAttribute{Optional: true},
			"metadata":     resourceschema.MapAttribute{Optional: true, ElementType: types.StringType},
			"credential_type": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Resolved credential type from auth.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"archived": resourceschema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"auth": resourceschema.SingleNestedAttribute{
				Required:  true,
				Sensitive: true,
				Description: "Authentication configuration.",
				Attributes: map[string]resourceschema.Attribute{
					"type": resourceschema.StringAttribute{
						Required:    true,
						Description: "Auth type, e.g. mcp_oauth.",
						Validators:  []validator.String{stringvalidator.OneOf("mcp_oauth")},
					},
					"mcp_server_url": resourceschema.StringAttribute{Optional: true, Description: "MCP server URL for OAuth."},
					"access_token":   resourceschema.StringAttribute{Optional: true, Sensitive: true, Description: "OAuth access token."},
					"refresh": resourceschema.SingleNestedAttribute{
						Optional:  true,
						Sensitive: true,
						Description: "OAuth refresh token configuration.",
						Attributes: map[string]resourceschema.Attribute{
							"client_id":           resourceschema.StringAttribute{Optional: true},
							"refresh_token":       resourceschema.StringAttribute{Optional: true, Sensitive: true},
							"token_endpoint":      resourceschema.StringAttribute{Optional: true},
							"token_endpoint_auth": resourceschema.StringAttribute{Optional: true, Description: "e.g. basic."},
							"scope":               resourceschema.StringAttribute{Optional: true},
						},
					},
				},
			},
		},
	}
}

func (r *vaultCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vaultCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, diags := buildCredentialPayload(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api credentialAPIModel
	if err := r.client.Post(ctx, fmt.Sprintf("/v1/vaults/%s/credentials", plan.VaultID.ValueString()), payload, &api); err != nil {
		resp.Diagnostics.AddError("Create credential failed", err.Error())
		return
	}
	state, diags := flattenCredentialState(ctx, plan, api)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vaultCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vaultCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api credentialAPIModel
	if err := r.client.Get(ctx, fmt.Sprintf("/v1/vaults/%s/credentials/%s", state.VaultID.ValueString(), state.ID.ValueString()), &api); err != nil {
		var nfe *NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read credential failed", err.Error())
		return
	}
	newState, diags := flattenCredentialState(ctx, state, api)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *vaultCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan vaultCredentialResourceModel
	var state vaultCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, diags := buildCredentialPayload(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api credentialAPIModel
	if err := r.client.Post(ctx, fmt.Sprintf("/v1/vaults/%s/credentials/%s", state.VaultID.ValueString(), state.ID.ValueString()), payload, &api); err != nil {
		resp.Diagnostics.AddError("Update credential failed", err.Error())
		return
	}
	newState, diags := flattenCredentialState(ctx, plan, api)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *vaultCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vaultCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var err error
	if r.archiveOnDestroy {
		err = r.client.Post(ctx, fmt.Sprintf("/v1/vaults/%s/credentials/%s/archive", state.VaultID.ValueString(), state.ID.ValueString()), map[string]any{}, nil)
	} else {
		err = r.client.Delete(ctx, fmt.Sprintf("/v1/vaults/%s/credentials/%s", state.VaultID.ValueString(), state.ID.ValueString()))
	}
	if err != nil {
		var nfe *NotFoundError
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError("Delete credential failed", err.Error())
	}
}

func (r *vaultCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: vault_id/credential_id")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vault_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func buildCredentialPayload(ctx context.Context, plan vaultCredentialResourceModel) (credentialRequestPayload, diag.Diagnostics) {
	var diags diag.Diagnostics
	meta, d := mapFromTF(ctx, plan.Metadata)
	diags.Append(d...)

	auth := expandCredentialAuth(ctx, plan.Auth, &diags)
	payload := credentialRequestPayload{Auth: auth, Metadata: meta}
	if !plan.DisplayName.IsNull() && !plan.DisplayName.IsUnknown() {
		payload.DisplayName = plan.DisplayName.ValueString()
	}
	return payload, diags
}

func expandCredentialAuth(ctx context.Context, obj types.Object, diags *diag.Diagnostics) credentialAuthAPI {
	if obj.IsNull() || obj.IsUnknown() {
		return credentialAuthAPI{}
	}
	var auth credentialAuthModel
	*diags = append(*diags, obj.As(ctx, &auth, basetypes.ObjectAsOptions{})...)

	result := credentialAuthAPI{
		Type: auth.Type.ValueString(),
	}
	if !auth.MCPServerURL.IsNull() && !auth.MCPServerURL.IsUnknown() {
		result.MCPServerURL = auth.MCPServerURL.ValueString()
	}
	if !auth.AccessToken.IsNull() && !auth.AccessToken.IsUnknown() {
		result.AccessToken = auth.AccessToken.ValueString()
	}
	if !auth.RefreshConfig.IsNull() && !auth.RefreshConfig.IsUnknown() {
		var refresh credentialRefreshModel
		*diags = append(*diags, auth.RefreshConfig.As(ctx, &refresh, basetypes.ObjectAsOptions{})...)
		r := credentialRefreshAPI{}
		if !refresh.ClientID.IsNull() && !refresh.ClientID.IsUnknown() {
			r.ClientID = refresh.ClientID.ValueString()
		}
		if !refresh.RefreshToken.IsNull() && !refresh.RefreshToken.IsUnknown() {
			r.RefreshToken = refresh.RefreshToken.ValueString()
		}
		if !refresh.TokenEndpoint.IsNull() && !refresh.TokenEndpoint.IsUnknown() {
			r.TokenEndpoint = refresh.TokenEndpoint.ValueString()
		}
		if !refresh.TokenEndpointAuth.IsNull() && !refresh.TokenEndpointAuth.IsUnknown() {
			r.TokenEndpointAuth = refresh.TokenEndpointAuth.ValueString()
		}
		if !refresh.Scope.IsNull() && !refresh.Scope.IsUnknown() {
			r.Scope = refresh.Scope.ValueString()
		}
		result.Refresh = &r
	}
	return result
}

func flattenCredentialState(ctx context.Context, prior vaultCredentialResourceModel, api credentialAPIModel) (vaultCredentialResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	// API does not echo back secrets; preserve auth from prior state to avoid perpetual diff.
	// After import, prior.Auth is null — build a partial auth object from the API response
	// so the resource state is not completely empty.
	auth := prior.Auth
	if auth.IsNull() || auth.IsUnknown() {
		refreshObj := types.ObjectNull(credentialRefreshAttrTypes())
		var d diag.Diagnostics
		auth, d = types.ObjectValue(credentialAuthAttrTypes(), map[string]attr.Value{
			"type":           stringOrNull(api.Auth.Type),
			"mcp_server_url": stringOrNull(api.Auth.MCPServerURL),
			"access_token":   types.StringNull(), // Not returned by API
			"refresh":        refreshObj,
		})
		diags.Append(d...)
	}

	state := vaultCredentialResourceModel{
		ID:             types.StringValue(api.ID),
		VaultID:        prior.VaultID,
		DisplayName:    stringOrNull(api.DisplayName),
		Auth:           auth,
		Archived:       types.BoolValue(api.ArchivedAt != nil),
		CredentialType: stringOrNull(api.Auth.Type),
	}
	meta, d := mapToTF(ctx, api.Metadata)
	diags.Append(d...)
	state.Metadata = meta
	return state, diags
}
