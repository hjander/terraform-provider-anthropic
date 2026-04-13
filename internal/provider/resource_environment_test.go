package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandEnvironmentPayload_Minimal(t *testing.T) {
	ctx := context.Background()

	netObj, d := types.ObjectValue(environmentNetworkingAttrTypes(), map[string]attr.Value{
		"type":                   types.StringValue("unrestricted"),
		"allow_mcp_servers":      types.BoolValue(false),
		"allow_package_managers": types.BoolValue(false),
		"allowed_hosts":          types.SetNull(types.StringType),
	})
	if d.HasError() {
		t.Fatal(d)
	}

	cfgObj, d := types.ObjectValue(map[string]attr.Type{
		"type":       types.StringType,
		"networking": types.ObjectType{AttrTypes: environmentNetworkingAttrTypes()},
		"packages":   types.ObjectType{AttrTypes: environmentPackagesAttrTypes()},
	}, map[string]attr.Value{
		"type":       types.StringValue("cloud"),
		"networking": netObj,
		"packages":   types.ObjectNull(environmentPackagesAttrTypes()),
	})
	if d.HasError() {
		t.Fatal(d)
	}

	plan := environmentResourceModel{
		Name:        types.StringValue("test-env"),
		Description: types.StringNull(),
		Metadata:    types.MapNull(types.StringType),
		Config:      cfgObj,
	}

	payload, diags := expandEnvironmentPayload(ctx, plan)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if payload.Name != "test-env" {
		t.Errorf("name=%v", payload.Name)
	}
	if payload.Config == nil {
		t.Fatal("config should not be nil")
	}
	if payload.Config.Type != "cloud" {
		t.Errorf("config.type=%v", payload.Config.Type)
	}
	if payload.Config.Networking.Type != "unrestricted" {
		t.Errorf("networking.type=%v", payload.Config.Networking.Type)
	}
}

func TestFlattenEnvironmentState_Minimal(t *testing.T) {
	api := environmentAPIModel{
		ID:   "env_123",
		Name: "my-env",
		Config: &environmentConfigAPI{
			Type: "cloud",
			Networking: environmentNetworkingAPI{
				Type: "unrestricted",
			},
		},
	}

	state, diags := flattenEnvironmentState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if state.ID.ValueString() != "env_123" {
		t.Errorf("id=%s", state.ID.ValueString())
	}
	if state.Name.ValueString() != "my-env" {
		t.Errorf("name=%s", state.Name.ValueString())
	}
	if state.Config.IsNull() {
		t.Error("config should not be null")
	}
}

func TestFlattenEnvironmentState_WithPackages(t *testing.T) {
	api := environmentAPIModel{
		ID:   "env_456",
		Name: "python-env",
		Config: &environmentConfigAPI{
			Type: "cloud",
			Networking: environmentNetworkingAPI{
				Type:                 "limited",
				AllowMCPServers:      true,
				AllowPackageManagers: true,
				AllowedHosts:         []string{"pypi.org"},
			},
			Packages: &environmentPackagesAPI{
				Type: "packages",
				PIP:  []string{"pandas==2.0"},
				NPM:  []string{"typescript@5"},
			},
		},
	}

	state, diags := flattenEnvironmentState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if state.Config.IsNull() {
		t.Fatal("config should not be null")
	}
}

func TestFlattenEnvironmentState_Archived(t *testing.T) {
	ts := "2025-01-01T00:00:00Z"
	api := environmentAPIModel{
		ID:         "env_789",
		Name:       "env",
		ArchivedAt: &ts,
		Config: &environmentConfigAPI{
			Type: "cloud",
			Networking: environmentNetworkingAPI{
				Type: "unrestricted",
			},
		},
	}

	state, diags := flattenEnvironmentState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !state.Archived.ValueBool() {
		t.Error("expected archived=true")
	}
}

func TestFlattenEnvironmentState_NilConfig(t *testing.T) {
	api := environmentAPIModel{
		ID:   "env_nil",
		Name: "no-config",
	}

	state, diags := flattenEnvironmentState(context.Background(), api)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !state.Config.IsNull() {
		t.Error("config should be null when API returns nil")
	}
}

func TestEnvironmentAttrTypes(t *testing.T) {
	net := environmentNetworkingAttrTypes()
	if len(net) != 4 {
		t.Errorf("expected 4 networking attrs, got %d", len(net))
	}
	pkg := environmentPackagesAttrTypes()
	if len(pkg) != 7 {
		t.Errorf("expected 7 package attrs, got %d", len(pkg))
	}
}
