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

type credentialAPIModel struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Auth        map[string]any    `json:"auth,omitempty"`
	ArchivedAt  *string           `json:"archived_at,omitempty"`
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
			"id":              resourceschema.StringAttribute{Computed: true},
			"vault_id":        resourceschema.StringAttribute{Required: true, Description: "Parent vault ID."},
			"display_name":    resourceschema.StringAttribute{Optional: true},
			"metadata":        resourceschema.MapAttribute{Optional: true, ElementType: types.StringType},
			"credential_type": resourceschema.StringAttribute{Computed: true, Description: "Resolved credential type from auth."},
			"archived":        resourceschema.BoolAttribute{Computed: true},
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
			// Resource was deleted outside Terraform; remove from state.
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
		resp.Diagnostics.AddError("Delete credential failed", err.Error())
	}
}

func (r *vaultCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: vault_id/credential_id (both required for the API path).
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: vault_id/credential_id")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vault_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func buildCredentialPayload(ctx context.Context, plan vaultCredentialResourceModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	meta, d := mapFromTF(ctx, plan.Metadata)
	diags.Append(d...)

	auth := expandCredentialAuth(ctx, plan.Auth, &diags)
	payload := map[string]any{"auth": auth}
	if !plan.DisplayName.IsNull() && !plan.DisplayName.IsUnknown() {
		payload["display_name"] = plan.DisplayName.ValueString()
	}
	if len(meta) > 0 {
		payload["metadata"] = meta
	}
	return payload, diags
}

func expandCredentialAuth(ctx context.Context, obj types.Object, diags *diag.Diagnostics) map[string]any {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	var auth credentialAuthModel
	*diags = append(*diags, obj.As(ctx, &auth, basetypes.ObjectAsOptions{})...)

	result := map[string]any{
		"type": auth.Type.ValueString(),
	}
	if !auth.MCPServerURL.IsNull() && !auth.MCPServerURL.IsUnknown() {
		result["mcp_server_url"] = auth.MCPServerURL.ValueString()
	}
	if !auth.AccessToken.IsNull() && !auth.AccessToken.IsUnknown() {
		result["access_token"] = auth.AccessToken.ValueString()
	}
	if !auth.RefreshConfig.IsNull() && !auth.RefreshConfig.IsUnknown() {
		var refresh credentialRefreshModel
		*diags = append(*diags, auth.RefreshConfig.As(ctx, &refresh, basetypes.ObjectAsOptions{})...)
		r := map[string]any{}
		if !refresh.ClientID.IsNull() && !refresh.ClientID.IsUnknown() {
			r["client_id"] = refresh.ClientID.ValueString()
		}
		if !refresh.RefreshToken.IsNull() && !refresh.RefreshToken.IsUnknown() {
			r["refresh_token"] = refresh.RefreshToken.ValueString()
		}
		if !refresh.TokenEndpoint.IsNull() && !refresh.TokenEndpoint.IsUnknown() {
			r["token_endpoint"] = refresh.TokenEndpoint.ValueString()
		}
		if !refresh.TokenEndpointAuth.IsNull() && !refresh.TokenEndpointAuth.IsUnknown() {
			r["token_endpoint_auth"] = refresh.TokenEndpointAuth.ValueString()
		}
		if !refresh.Scope.IsNull() && !refresh.Scope.IsUnknown() {
			r["scope"] = refresh.Scope.ValueString()
		}
		result["refresh"] = r
	}
	return result
}

func flattenCredentialState(ctx context.Context, prior vaultCredentialResourceModel, api credentialAPIModel) (vaultCredentialResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := vaultCredentialResourceModel{
		ID:             types.StringValue(api.ID),
		VaultID:        prior.VaultID,
		DisplayName:    stringOrNull(api.DisplayName),
		// API does not echo back secrets; preserve auth from prior state to avoid perpetual diff.
		Auth: prior.Auth,
		Archived:       types.BoolValue(api.ArchivedAt != nil),
		CredentialType: stringOrNull(anyString(api.Auth["type"])),
	}
	meta, d := mapToTF(ctx, api.Metadata)
	diags.Append(d...)
	state.Metadata = meta
	return state, diags
}
