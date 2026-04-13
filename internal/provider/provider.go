package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	defaultBaseURL       = "https://api.anthropic.com"
	defaultAnthropicVer  = "2023-06-01"
	defaultManagedAgents = "managed-agents-2026-04-01"
)

type managedAgentsProvider struct{}

type managedAgentsProviderModel struct {
	APIKey            types.String `tfsdk:"api_key"`
	BaseURL           types.String `tfsdk:"base_url"`
	AnthropicVersion  types.String `tfsdk:"anthropic_version"`
	ManagedAgentsBeta types.String `tfsdk:"managed_agents_beta"`
	ArchiveOnDestroy  types.Bool   `tfsdk:"archive_on_destroy"`
}

type providerData struct {
	client           *Client
	archiveOnDestroy bool
}

var _ provider.Provider = (*managedAgentsProvider)(nil)

func New() provider.Provider {
	return &managedAgentsProvider{}
}

func (p *managedAgentsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "anthropic"
}

func (p *managedAgentsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{
		Description: "Terraform provider for Anthropic Claude Managed Agents.",
		Attributes: map[string]providerschema.Attribute{
			"api_key": providerschema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Anthropic API key. Can also be supplied via ANTHROPIC_API_KEY.",
			},
			"base_url": providerschema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("Anthropic API base URL. Defaults to %s.", defaultBaseURL),
			},
			"anthropic_version": providerschema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("Anthropic API version header. Defaults to %s.", defaultAnthropicVer),
			},
			"managed_agents_beta": providerschema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("Managed Agents beta header. Defaults to %s.", defaultManagedAgents),
			},
			"archive_on_destroy": providerschema.BoolAttribute{
				Optional:    true,
				Description: "When true, resources are archived on destroy when the API supports archiving. Otherwise they are hard-deleted where possible.",
			},
		},
	}
}

func (p *managedAgentsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data managedAgentsProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if !data.APIKey.IsNull() && !data.APIKey.IsUnknown() {
		apiKey = data.APIKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddError("Missing Anthropic API key", "Set api_key in the provider block or ANTHROPIC_API_KEY in the environment.")
		return
	}

	baseURL := defaultBaseURL
	if !data.BaseURL.IsNull() && !data.BaseURL.IsUnknown() && data.BaseURL.ValueString() != "" {
		baseURL = data.BaseURL.ValueString()
	}

	anthropicVersion := defaultAnthropicVer
	if !data.AnthropicVersion.IsNull() && !data.AnthropicVersion.IsUnknown() && data.AnthropicVersion.ValueString() != "" {
		anthropicVersion = data.AnthropicVersion.ValueString()
	}

	managedAgentsBeta := defaultManagedAgents
	if !data.ManagedAgentsBeta.IsNull() && !data.ManagedAgentsBeta.IsUnknown() && data.ManagedAgentsBeta.ValueString() != "" {
		managedAgentsBeta = data.ManagedAgentsBeta.ValueString()
	}

	archiveOnDestroy := true
	if !data.ArchiveOnDestroy.IsNull() && !data.ArchiveOnDestroy.IsUnknown() {
		archiveOnDestroy = data.ArchiveOnDestroy.ValueBool()
	}

	pd := &providerData{
		client: NewClient(ClientConfig{
			BaseURL:           baseURL,
			APIKey:            apiKey,
			AnthropicVersion:  anthropicVersion,
			ManagedAgentsBeta: managedAgentsBeta,
		}),
		archiveOnDestroy: archiveOnDestroy,
	}

	resp.DataSourceData = pd
	resp.ResourceData = pd
}

func (p *managedAgentsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEnvironmentResource,
		NewAgentResource,
		NewVaultResource,
		NewVaultCredentialResource,
	}
}

func (p *managedAgentsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewEnvironmentDataSource,
		NewAgentDataSource,
		NewVaultDataSource,
		NewVaultCredentialDataSource,
	}
}
