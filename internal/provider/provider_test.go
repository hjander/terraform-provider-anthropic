package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"anthropic": providerserver.NewProtocol6WithError(New()),
}

func TestProviderMetadata(t *testing.T) {
	p := &managedAgentsProvider{}
	var resp provider.MetadataResponse
	p.Metadata(context.Background(), provider.MetadataRequest{}, &resp)
	if resp.TypeName != "anthropic" {
		t.Errorf("type_name=%s, want anthropic", resp.TypeName)
	}
}

func TestProviderSchema(t *testing.T) {
	p := &managedAgentsProvider{}
	var resp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &resp)

	expected := []string{"api_key", "base_url", "anthropic_version", "managed_agents_beta", "archive_on_destroy"}
	for _, name := range expected {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("missing attribute %s", name)
		}
	}
}

func TestProviderResources(t *testing.T) {
	p := &managedAgentsProvider{}
	resources := p.Resources(context.Background())
	if len(resources) != 4 {
		t.Errorf("expected 4 resources, got %d", len(resources))
	}
}

func TestProviderDataSources(t *testing.T) {
	p := &managedAgentsProvider{}
	ds := p.DataSources(context.Background())
	if len(ds) != 4 {
		t.Errorf("expected 4 data sources, got %d", len(ds))
	}
}

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
}
