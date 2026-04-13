package provider

import (
	"encoding/json"
	"testing"
)

func TestEnvironmentNetworkingAPI_BooleanFalsePreserved(t *testing.T) {
	f := false
	net := environmentNetworkingAPI{
		Type:                 "limited",
		AllowMCPServers:      &f,
		AllowPackageManagers: &f,
	}
	data, err := json.Marshal(net)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := raw["allow_mcp_servers"]; !ok {
		t.Error("allow_mcp_servers=false was omitted from JSON; expected it to be present")
	}
	if _, ok := raw["allow_package_managers"]; !ok {
		t.Error("allow_package_managers=false was omitted from JSON; expected it to be present")
	}
}

func TestEnvironmentNetworkingAPI_BooleanTruePreserved(t *testing.T) {
	tr := true
	net := environmentNetworkingAPI{
		Type:                 "limited",
		AllowMCPServers:      &tr,
		AllowPackageManagers: &tr,
	}
	data, err := json.Marshal(net)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, ok := raw["allow_mcp_servers"]; !ok || v != true {
		t.Errorf("allow_mcp_servers: got %v, want true", v)
	}
	if v, ok := raw["allow_package_managers"]; !ok || v != true {
		t.Errorf("allow_package_managers: got %v, want true", v)
	}
}

func TestEnvironmentNetworkingAPI_NilBoolsOmitted(t *testing.T) {
	net := environmentNetworkingAPI{
		Type: "unrestricted",
	}
	data, err := json.Marshal(net)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := raw["allow_mcp_servers"]; ok {
		t.Error("allow_mcp_servers should be omitted for unrestricted type")
	}
	if _, ok := raw["allow_package_managers"]; ok {
		t.Error("allow_package_managers should be omitted for unrestricted type")
	}
}
