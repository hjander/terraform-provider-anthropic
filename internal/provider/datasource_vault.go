package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type vaultDataSource struct {
	client *Client
}

var _ datasource.DataSource = (*vaultDataSource)(nil)

func NewVaultDataSource() datasource.DataSource { return &vaultDataSource{} }

func (d *vaultDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_vault"
}

func (d *vaultDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *vaultDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		Description: "Look up a managed vault by ID.",
		Attributes: map[string]datasourceschema.Attribute{
			"id":           datasourceschema.StringAttribute{Required: true, Description: "Vault ID."},
			"display_name": datasourceschema.StringAttribute{Computed: true},
			"metadata":     datasourceschema.MapAttribute{Computed: true, ElementType: types.StringType},
			"archived":     datasourceschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *vaultDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vaultResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api vaultAPIModel
	if err := d.client.Get(ctx, fmt.Sprintf("/v1/vaults/%s", config.ID.ValueString()), &api); err != nil {
		resp.Diagnostics.AddError("Read vault failed", err.Error())
		return
	}

	state, diags := flattenVaultState(ctx, api)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
