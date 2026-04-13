package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFlattenVaultState(t *testing.T) {
	api := vaultAPIModel{
		ID:          "vault_123",
		DisplayName: "test vault",
		Metadata:    map[string]string{"env": "prod"},
	}

	state, diags := flattenVaultState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if state.ID.ValueString() != "vault_123" {
		t.Errorf("id=%s", state.ID.ValueString())
	}
	if state.DisplayName.ValueString() != "test vault" {
		t.Errorf("display_name=%s", state.DisplayName.ValueString())
	}
	if state.Archived.ValueBool() {
		t.Error("expected not archived")
	}
	if state.Metadata.IsNull() {
		t.Error("expected metadata to be set")
	}
}

func TestFlattenVaultState_Archived(t *testing.T) {
	ts := "2025-01-01T00:00:00Z"
	api := vaultAPIModel{
		ID:          "vault_123",
		DisplayName: "vault",
		ArchivedAt:  &ts,
	}

	state, diags := flattenVaultState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !state.Archived.ValueBool() {
		t.Error("expected archived=true")
	}
}

func TestFlattenVaultState_NoMetadata(t *testing.T) {
	api := vaultAPIModel{
		ID:          "vault_456",
		DisplayName: "empty meta",
	}

	state, diags := flattenVaultState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !state.Metadata.IsNull() {
		t.Error("expected null metadata for empty map")
	}
}

func TestVaultResourceSchema(t *testing.T) {
	r := &vaultResource{}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	attrs := resp.Schema.Attributes
	required := []string{"display_name"}
	for _, name := range required {
		a, ok := attrs[name]
		if !ok {
			t.Errorf("missing attribute %s", name)
			continue
		}
		sa, ok := a.(resourceschema.StringAttribute)
		if !ok {
			t.Errorf("%s is not StringAttribute", name)
			continue
		}
		if !sa.Required {
			t.Errorf("%s should be required", name)
		}
	}

	computed := []string{"id", "archived"}
	for _, name := range computed {
		_, ok := attrs[name]
		if !ok {
			t.Errorf("missing computed attribute %s", name)
		}
	}
}

func TestVaultMetadata(t *testing.T) {
	r := &vaultResource{}
	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "anthropic"}, &resp)
	if resp.TypeName != "anthropic_managed_vault" {
		t.Errorf("type_name=%s", resp.TypeName)
	}
}

func TestVaultConfigure_NilProvider(t *testing.T) {
	r := &vaultResource{}
	r.Configure(context.Background(), resource.ConfigureRequest{}, &resource.ConfigureResponse{})
	if r.client != nil {
		t.Error("client should be nil when no provider data")
	}
}

func TestVaultConfigure_WithProvider(t *testing.T) {
	c := NewClient(ClientConfig{BaseURL: "http://test", APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	pd := &providerData{client: c, archiveOnDestroy: true}

	r := &vaultResource{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: pd}, &resource.ConfigureResponse{})
	if r.client == nil {
		t.Error("client should be set")
	}
	if !r.archiveOnDestroy {
		t.Error("archiveOnDestroy should be true")
	}
}

func TestVaultPayload(t *testing.T) {
	ctx := context.Background()
	meta, _ := types.MapValueFrom(ctx, types.StringType, map[string]string{"env": "test"})

	plan := vaultResourceModel{
		DisplayName: types.StringValue("my vault"),
		Metadata:    meta,
	}

	m, d := mapFromTF(ctx, plan.Metadata)
	if d.HasError() {
		t.Fatal(d)
	}

	payload := map[string]any{"display_name": plan.DisplayName.ValueString()}
	if len(m) > 0 {
		payload["metadata"] = m
	}

	if payload["display_name"] != "my vault" {
		t.Errorf("display_name=%v", payload["display_name"])
	}
	if payload["metadata"].(map[string]string)["env"] != "test" {
		t.Error("metadata not set correctly")
	}
}
