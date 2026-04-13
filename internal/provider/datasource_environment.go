package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type environmentDataSource struct {
	client *Client
}

type environmentDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Metadata    types.Map    `tfsdk:"metadata"`
	Config      types.Object `tfsdk:"config"`
	Archived    types.Bool   `tfsdk:"archived"`
}

var _ datasource.DataSource = (*environmentDataSource)(nil)

func NewEnvironmentDataSource() datasource.DataSource { return &environmentDataSource{} }

func (d *environmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_environment"
}

func (d *environmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *environmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		Description: "Look up a managed environment by ID.",
		Attributes: map[string]datasourceschema.Attribute{
			"id":          datasourceschema.StringAttribute{Required: true, Description: "Environment ID."},
			"name":        datasourceschema.StringAttribute{Computed: true},
			"description": datasourceschema.StringAttribute{Computed: true},
			"metadata":    datasourceschema.MapAttribute{Computed: true, ElementType: types.StringType},
			"archived":    datasourceschema.BoolAttribute{Computed: true},
			"config": datasourceschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]datasourceschema.Attribute{
					"type": datasourceschema.StringAttribute{Computed: true},
					"networking": datasourceschema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]datasourceschema.Attribute{
							"type":                   datasourceschema.StringAttribute{Computed: true},
							"allow_mcp_servers":      datasourceschema.BoolAttribute{Computed: true},
							"allow_package_managers": datasourceschema.BoolAttribute{Computed: true},
							"allowed_hosts":          datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType},
						},
					},
					"packages": datasourceschema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]datasourceschema.Attribute{
							"type":  datasourceschema.StringAttribute{Computed: true},
							"apt":   datasourceschema.ListAttribute{Computed: true, ElementType: types.StringType},
							"cargo": datasourceschema.ListAttribute{Computed: true, ElementType: types.StringType},
							"gem":   datasourceschema.ListAttribute{Computed: true, ElementType: types.StringType},
							"go":    datasourceschema.ListAttribute{Computed: true, ElementType: types.StringType},
							"npm":   datasourceschema.ListAttribute{Computed: true, ElementType: types.StringType},
							"pip":   datasourceschema.ListAttribute{Computed: true, ElementType: types.StringType},
						},
					},
				},
			},
		},
	}
}

func (d *environmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config environmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "reading environment data source", map[string]any{"id": config.ID.ValueString()})
	var api environmentAPIModel
	if err := d.client.Get(ctx, fmt.Sprintf("/v1/environments/%s", config.ID.ValueString()), &api); err != nil {
		resp.Diagnostics.AddError("Read environment failed", err.Error())
		return
	}

	flat, diags := flattenEnvironmentState(ctx, api)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state := environmentDataSourceModel{
		ID:          flat.ID,
		Name:        flat.Name,
		Description: flat.Description,
		Metadata:    flat.Metadata,
		Config:      flat.Config,
		Archived:    flat.Archived,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
