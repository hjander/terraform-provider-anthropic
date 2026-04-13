package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandCredentialAuth_Minimal(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	authObj, d := types.ObjectValue(credentialAuthAttrTypes(), map[string]attr.Value{
		"type":           types.StringValue("mcp_oauth"),
		"mcp_server_url": types.StringValue("https://mcp.example.com"),
		"access_token":   types.StringValue("token123"),
		"refresh":        types.ObjectNull(credentialRefreshAttrTypes()),
	})
	if d.HasError() {
		t.Fatal(d)
	}

	result := expandCredentialAuth(ctx, authObj, &diags)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if result["type"] != "mcp_oauth" {
		t.Errorf("type=%v", result["type"])
	}
	if result["mcp_server_url"] != "https://mcp.example.com" {
		t.Errorf("mcp_server_url=%v", result["mcp_server_url"])
	}
	if result["access_token"] != "token123" {
		t.Errorf("access_token=%v", result["access_token"])
	}
	if _, ok := result["refresh"]; ok {
		t.Error("refresh should not be set when null")
	}
}

func TestExpandCredentialAuth_WithRefresh(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	refreshObj, d := types.ObjectValue(credentialRefreshAttrTypes(), map[string]attr.Value{
		"client_id":           types.StringValue("client_abc"),
		"refresh_token":       types.StringValue("rt_xyz"),
		"token_endpoint":      types.StringValue("https://auth.example.com/token"),
		"token_endpoint_auth": types.StringValue("basic"),
		"scope":               types.StringValue("repo read:org"),
	})
	if d.HasError() {
		t.Fatal(d)
	}

	authObj, d := types.ObjectValue(credentialAuthAttrTypes(), map[string]attr.Value{
		"type":           types.StringValue("mcp_oauth"),
		"mcp_server_url": types.StringValue("https://mcp.example.com"),
		"access_token":   types.StringValue("token123"),
		"refresh":        refreshObj,
	})
	if d.HasError() {
		t.Fatal(d)
	}

	result := expandCredentialAuth(ctx, authObj, &diags)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	refresh, ok := result["refresh"].(map[string]any)
	if !ok {
		t.Fatalf("refresh should be map, got %T", result["refresh"])
	}
	if refresh["client_id"] != "client_abc" {
		t.Errorf("client_id=%v", refresh["client_id"])
	}
	if refresh["refresh_token"] != "rt_xyz" {
		t.Errorf("refresh_token=%v", refresh["refresh_token"])
	}
	if refresh["scope"] != "repo read:org" {
		t.Errorf("scope=%v", refresh["scope"])
	}
}

func TestExpandCredentialAuth_Null(t *testing.T) {
	var diags diag.Diagnostics
	result := expandCredentialAuth(context.Background(), types.ObjectNull(credentialAuthAttrTypes()), &diags)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFlattenCredentialState(t *testing.T) {
	prior := vaultCredentialResourceModel{
		VaultID: types.StringValue("vault_123"),
		Auth: func() types.Object {
			obj, _ := types.ObjectValue(credentialAuthAttrTypes(), map[string]attr.Value{
				"type":           types.StringValue("mcp_oauth"),
				"mcp_server_url": types.StringValue("https://mcp.example.com"),
				"access_token":   types.StringValue("secret"),
				"refresh":        types.ObjectNull(credentialRefreshAttrTypes()),
			})
			return obj
		}(),
	}
	api := credentialAPIModel{
		ID:          "cred_456",
		DisplayName: "GitHub OAuth",
		Auth:        map[string]any{"type": "mcp_oauth"},
	}

	state, diags := flattenCredentialState(context.Background(), prior, api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if state.ID.ValueString() != "cred_456" {
		t.Errorf("id=%s", state.ID.ValueString())
	}
	if state.CredentialType.ValueString() != "mcp_oauth" {
		t.Errorf("credential_type=%s", state.CredentialType.ValueString())
	}
	if state.Auth.IsNull() {
		t.Error("auth should be preserved from prior")
	}
}

func TestFlattenCredentialState_Archived(t *testing.T) {
	ts := "2025-01-01T00:00:00Z"
	prior := vaultCredentialResourceModel{
		VaultID: types.StringValue("vault_123"),
		Auth:    types.ObjectNull(credentialAuthAttrTypes()),
	}
	api := credentialAPIModel{
		ID:         "cred_789",
		ArchivedAt: &ts,
		Auth:       map[string]any{"type": "mcp_oauth"},
	}

	state, diags := flattenCredentialState(context.Background(), prior, api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !state.Archived.ValueBool() {
		t.Error("expected archived=true")
	}
}

func TestBuildCredentialPayload(t *testing.T) {
	ctx := context.Background()

	authObj, d := types.ObjectValue(credentialAuthAttrTypes(), map[string]attr.Value{
		"type":           types.StringValue("mcp_oauth"),
		"mcp_server_url": types.StringValue("https://mcp.example.com"),
		"access_token":   types.StringValue("tok"),
		"refresh":        types.ObjectNull(credentialRefreshAttrTypes()),
	})
	if d.HasError() {
		t.Fatal(d)
	}

	plan := vaultCredentialResourceModel{
		VaultID:     types.StringValue("vault_123"),
		DisplayName: types.StringValue("Test Cred"),
		Metadata:    types.MapNull(types.StringType),
		Auth:        authObj,
	}

	payload, diags := buildCredentialPayload(ctx, plan)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if payload["display_name"] != "Test Cred" {
		t.Errorf("display_name=%v", payload["display_name"])
	}
	auth, ok := payload["auth"].(map[string]any)
	if !ok {
		t.Fatalf("auth should be map, got %T", payload["auth"])
	}
	if auth["type"] != "mcp_oauth" {
		t.Errorf("auth.type=%v", auth["type"])
	}
}

func TestCredentialAttrTypes(t *testing.T) {
	auth := credentialAuthAttrTypes()
	if len(auth) != 4 {
		t.Errorf("expected 4 auth attrs, got %d", len(auth))
	}
	refresh := credentialRefreshAttrTypes()
	if len(refresh) != 5 {
		t.Errorf("expected 5 refresh attrs, got %d", len(refresh))
	}
}

func TestCredentialResourceSchema(t *testing.T) {
	r := &vaultCredentialResource{}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	attrs := resp.Schema.Attributes
	if _, ok := attrs["auth"]; !ok {
		t.Error("missing auth attribute")
	}
	if _, ok := attrs["vault_id"]; !ok {
		t.Error("missing vault_id attribute")
	}
}

func TestCredentialMetadata(t *testing.T) {
	r := &vaultCredentialResource{}
	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "anthropic"}, &resp)
	if resp.TypeName != "anthropic_managed_vault_credential" {
		t.Errorf("type_name=%s", resp.TypeName)
	}
}
