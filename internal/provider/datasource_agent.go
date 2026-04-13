package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type agentDataSource struct {
	client *Client
}

type agentDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	ModelID     types.String `tfsdk:"model_id"`
	ModelSpeed  types.String `tfsdk:"model_speed"`
	System      types.String `tfsdk:"system"`
	Metadata    types.Map    `tfsdk:"metadata"`
	MCPServers  types.List   `tfsdk:"mcp_servers"`
	Skills      types.List   `tfsdk:"skills"`
	Tools       types.List   `tfsdk:"tools"`
	Version     types.Int64  `tfsdk:"version"`
	Archived    types.Bool   `tfsdk:"archived"`
}

var _ datasource.DataSource = (*agentDataSource)(nil)

func NewAgentDataSource() datasource.DataSource { return &agentDataSource{} }

func (d *agentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_agent"
}

func (d *agentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *agentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		Description: "Look up a managed agent by ID.",
		Attributes: map[string]datasourceschema.Attribute{
			"id":          datasourceschema.StringAttribute{Required: true, Description: "Agent ID."},
			"name":        datasourceschema.StringAttribute{Computed: true},
			"description": datasourceschema.StringAttribute{Computed: true},
			"model_id":    datasourceschema.StringAttribute{Computed: true, Description: "Model identifier."},
			"model_speed": datasourceschema.StringAttribute{Computed: true, Description: "Model speed: standard or fast."},
			"system":      datasourceschema.StringAttribute{Computed: true, Description: "System prompt."},
			"metadata":    datasourceschema.MapAttribute{Computed: true, ElementType: types.StringType},
			"version":     datasourceschema.Int64Attribute{Computed: true},
			"archived":    datasourceschema.BoolAttribute{Computed: true},
			"mcp_servers": datasourceschema.ListNestedAttribute{
				Computed: true,
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: map[string]datasourceschema.Attribute{
						"name": datasourceschema.StringAttribute{Computed: true},
						"type": datasourceschema.StringAttribute{Computed: true},
						"url":  datasourceschema.StringAttribute{Computed: true},
					},
				},
			},
			"skills": datasourceschema.ListNestedAttribute{
				Computed: true,
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: map[string]datasourceschema.Attribute{
						"type":     datasourceschema.StringAttribute{Computed: true},
						"skill_id": datasourceschema.StringAttribute{Computed: true},
						"version":  datasourceschema.StringAttribute{Computed: true},
					},
				},
			},
			"tools": datasourceschema.ListNestedAttribute{
				Computed: true,
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: map[string]datasourceschema.Attribute{
						"type":            datasourceschema.StringAttribute{Computed: true},
						"name":            datasourceschema.StringAttribute{Computed: true},
						"description":     datasourceschema.StringAttribute{Computed: true},
						"input_schema":    datasourceschema.StringAttribute{Computed: true},
						"mcp_server_name": datasourceschema.StringAttribute{Computed: true},
						"default_config": datasourceschema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]datasourceschema.Attribute{
								"name":              datasourceschema.StringAttribute{Computed: true},
								"enabled":           datasourceschema.BoolAttribute{Computed: true},
								"permission_policy": datasourceschema.StringAttribute{Computed: true},
							},
						},
						"configs": datasourceschema.ListNestedAttribute{
							Computed: true,
							NestedObject: datasourceschema.NestedAttributeObject{
								Attributes: map[string]datasourceschema.Attribute{
									"name":              datasourceschema.StringAttribute{Computed: true},
									"enabled":           datasourceschema.BoolAttribute{Computed: true},
									"permission_policy": datasourceschema.StringAttribute{Computed: true},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *agentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config agentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api agentAPIModel
	if err := d.client.Get(ctx, fmt.Sprintf("/v1/agents/%s", config.ID.ValueString()), &api); err != nil {
		resp.Diagnostics.AddError("Read agent failed", err.Error())
		return
	}

	flat, diags := flattenAgentState(ctx, api)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state := agentDataSourceModel{
		ID:          flat.ID,
		Name:        flat.Name,
		Description: flat.Description,
		ModelID:     flat.ModelID,
		ModelSpeed:  flat.ModelSpeed,
		System:      flat.System,
		Metadata:    flat.Metadata,
		MCPServers:  flat.MCPServers,
		Skills:      flat.Skills,
		Tools:       flat.Tools,
		Version:     flat.Version,
		Archived:    flat.Archived,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
