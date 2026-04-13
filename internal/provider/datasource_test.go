package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func TestAgentDataSource_Metadata(t *testing.T) {
	ds := &agentDataSource{}
	var resp datasource.MetadataResponse
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "anthropic"}, &resp)
	if resp.TypeName != "anthropic_managed_agent" {
		t.Errorf("type_name=%s", resp.TypeName)
	}
}

func TestAgentDataSource_Schema(t *testing.T) {
	ds := &agentDataSource{}
	var resp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	expected := []string{"id", "name", "description", "model_id", "model_speed", "system", "metadata", "version", "archived", "mcp_servers", "skills", "tools"}
	for _, name := range expected {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("missing attribute %s", name)
		}
	}
}

func TestAgentDataSource_Configure_NilProvider(t *testing.T) {
	ds := &agentDataSource{}
	ds.Configure(context.Background(), datasource.ConfigureRequest{}, &datasource.ConfigureResponse{})
	if ds.client != nil {
		t.Error("client should be nil when no provider data")
	}
}

func TestEnvironmentDataSource_Metadata(t *testing.T) {
	ds := &environmentDataSource{}
	var resp datasource.MetadataResponse
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "anthropic"}, &resp)
	if resp.TypeName != "anthropic_managed_environment" {
		t.Errorf("type_name=%s", resp.TypeName)
	}
}

func TestEnvironmentDataSource_Schema(t *testing.T) {
	ds := &environmentDataSource{}
	var resp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	expected := []string{"id", "name", "description", "metadata", "archived", "config"}
	for _, name := range expected {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("missing attribute %s", name)
		}
	}
}

func TestVaultDataSource_Metadata(t *testing.T) {
	ds := &vaultDataSource{}
	var resp datasource.MetadataResponse
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "anthropic"}, &resp)
	if resp.TypeName != "anthropic_managed_vault" {
		t.Errorf("type_name=%s", resp.TypeName)
	}
}

func TestVaultDataSource_Schema(t *testing.T) {
	ds := &vaultDataSource{}
	var resp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	expected := []string{"id", "display_name", "metadata", "archived"}
	for _, name := range expected {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("missing attribute %s", name)
		}
	}
}

func TestVaultCredentialDataSource_Metadata(t *testing.T) {
	ds := &vaultCredentialDataSource{}
	var resp datasource.MetadataResponse
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "anthropic"}, &resp)
	if resp.TypeName != "anthropic_managed_vault_credential" {
		t.Errorf("type_name=%s", resp.TypeName)
	}
}

func TestVaultCredentialDataSource_Schema(t *testing.T) {
	ds := &vaultCredentialDataSource{}
	var resp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	expected := []string{"id", "vault_id", "display_name", "metadata", "credential_type", "archived"}
	for _, name := range expected {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("missing attribute %s", name)
		}
	}
}
