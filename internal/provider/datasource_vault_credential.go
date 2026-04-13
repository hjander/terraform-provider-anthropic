package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type vaultCredentialDataSource struct {
	client *Client
}

type vaultCredentialDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	VaultID        types.String `tfsdk:"vault_id"`
	DisplayName    types.String `tfsdk:"display_name"`
	Metadata       types.Map    `tfsdk:"metadata"`
	CredentialType types.String `tfsdk:"credential_type"`
	Archived       types.Bool   `tfsdk:"archived"`
}

var _ datasource.DataSource = (*vaultCredentialDataSource)(nil)

func NewVaultCredentialDataSource() datasource.DataSource { return &vaultCredentialDataSource{} }

func (d *vaultCredentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_vault_credential"
}

func (d *vaultCredentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *providerData, got %T", req.ProviderData))
		return
	}
	d.client = pd.client
}

func (d *vaultCredentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		Description: "Look up a vault credential by vault ID and credential ID. Sensitive auth fields are not returned.",
		Attributes: map[string]datasourceschema.Attribute{
			"id":              datasourceschema.StringAttribute{Required: true, Description: "Credential ID."},
			"vault_id":        datasourceschema.StringAttribute{Required: true, Description: "Parent vault ID."},
			"display_name":    datasourceschema.StringAttribute{Computed: true},
			"metadata":        datasourceschema.MapAttribute{Computed: true, ElementType: types.StringType},
			"credential_type": datasourceschema.StringAttribute{Computed: true, Description: "Resolved credential type from auth."},
			"archived":        datasourceschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *vaultCredentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vaultCredentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "reading vault credential data source", map[string]any{"id": config.ID.ValueString()})
	var api credentialAPIModel
	if err := d.client.Get(ctx, fmt.Sprintf("/v1/vaults/%s/credentials/%s", config.VaultID.ValueString(), config.ID.ValueString()), &api); err != nil {
		resp.Diagnostics.AddError("Read credential failed", err.Error())
		return
	}

	meta, diags := mapToTF(ctx, api.Metadata)
	resp.Diagnostics.Append(diags...)

	state := vaultCredentialDataSourceModel{
		ID:             types.StringValue(api.ID),
		VaultID:        config.VaultID,
		DisplayName:    stringOrNull(api.DisplayName),
		Metadata:       meta,
		CredentialType: stringOrNull(api.Auth.Type),
		Archived:       types.BoolValue(api.ArchivedAt != nil),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
