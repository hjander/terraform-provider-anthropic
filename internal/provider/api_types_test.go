package provider

import (
	"encoding/json"
	"testing"
)

func TestEnvironmentNetworkingAPI_BooleanFalsePreserved(t *testing.T) {
	n := environmentNetworkingAPI{
		Type:                 "limited",
		AllowMCPServers:      false,
		AllowPackageManagers: false,
	}

	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if _, ok := m["allow_mcp_servers"]; !ok {
		t.Errorf("expected key 'allow_mcp_servers' to be present in JSON when set to false, but it was omitted")
	}
	if _, ok := m["allow_package_managers"]; !ok {
		t.Errorf("expected key 'allow_package_managers' to be present in JSON when set to false, but it was omitted")
	}
}

func TestEnvironmentNetworkingAPI_BooleanTruePreserved(t *testing.T) {
	n := environmentNetworkingAPI{
		Type:                 "limited",
		AllowMCPServers:      true,
		AllowPackageManagers: true,
	}

	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if v, ok := m["allow_mcp_servers"]; !ok {
		t.Errorf("expected key 'allow_mcp_servers' to be present in JSON when set to true, but it was omitted")
	} else if v != true {
		t.Errorf("expected 'allow_mcp_servers' to be true, got %v", v)
	}

	if v, ok := m["allow_package_managers"]; !ok {
		t.Errorf("expected key 'allow_package_managers' to be present in JSON when set to true, but it was omitted")
	} else if v != true {
		t.Errorf("expected 'allow_package_managers' to be true, got %v", v)
	}
}
