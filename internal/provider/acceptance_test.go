package provider

// Integration tests: these exercise the raw HTTP client against the live API.
// They do not go through the Terraform provider binary or plan/apply lifecycle.

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func skipUnlessAcceptance(t *testing.T) {
	t.Helper()
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set; skipping integration test")
	}
}

func testClient(t *testing.T) *Client {
	t.Helper()
	return NewClient(ClientConfig{
		BaseURL:           "https://api.anthropic.com",
		APIKey:            os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicVersion:  "2023-06-01",
		ManagedAgentsBeta: "managed-agents-2026-04-01",
	})
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-tf-test-%d", prefix, time.Now().UnixMilli())
}

// ---------- Environment CRUD ----------

func TestIntegrationEnvironment_CRUD(t *testing.T) {
	skipUnlessAcceptance(t)
	ctx := context.Background()
	c := testClient(t)

	// CREATE
	createPayload := map[string]any{
		"name":        uniqueName("env"),
		"description": "acceptance test environment",
		"config": map[string]any{
			"type": "cloud",
			"networking": map[string]any{
				"type": "unrestricted",
			},
		},
	}

	var env environmentAPIModel
	if err := c.Post(ctx, "/v1/environments", createPayload, &env); err != nil {
		t.Fatalf("create environment: %v", err)
	}
	t.Logf("Created environment: id=%s name=%s", env.ID, env.Name)

	if env.ID == "" {
		t.Fatal("environment ID is empty")
	}
	if env.Name != createPayload["name"] {
		t.Errorf("name=%s, want %s", env.Name, createPayload["name"])
	}

	// READ
	var readEnv environmentAPIModel
	if err := c.Get(ctx, fmt.Sprintf("/v1/environments/%s", env.ID), &readEnv); err != nil {
		t.Fatalf("read environment: %v", err)
	}
	if readEnv.ID != env.ID {
		t.Errorf("read id=%s, want %s", readEnv.ID, env.ID)
	}
	t.Logf("Read environment: id=%s name=%s", readEnv.ID, readEnv.Name)

	// UPDATE
	updatePayload := map[string]any{
		"name":        env.Name + "-updated",
		"description": "updated description",
		"config": map[string]any{
			"type": "cloud",
			"networking": map[string]any{
				"type":                   "limited",
				"allow_mcp_servers":      true,
				"allow_package_managers": true,
			},
		},
	}

	var updatedEnv environmentAPIModel
	if err := c.Post(ctx, fmt.Sprintf("/v1/environments/%s", env.ID), updatePayload, &updatedEnv); err != nil {
		t.Fatalf("update environment: %v", err)
	}
	if updatedEnv.Description != "updated description" {
		t.Errorf("description=%s, want 'updated description'", updatedEnv.Description)
	}
	t.Logf("Updated environment: id=%s desc=%s", updatedEnv.ID, updatedEnv.Description)

	// DELETE
	if err := c.Delete(ctx, fmt.Sprintf("/v1/environments/%s", env.ID)); err != nil {
		t.Fatalf("delete environment: %v", err)
	}
	t.Logf("Deleted environment: id=%s", env.ID)
}

func TestIntegrationEnvironment_WithPackages(t *testing.T) {
	skipUnlessAcceptance(t)
	ctx := context.Background()
	c := testClient(t)

	payload := map[string]any{
		"name": uniqueName("env-pkg"),
		"config": map[string]any{
			"type": "cloud",
			"networking": map[string]any{
				"type":                   "limited",
				"allow_package_managers": true,
			},
			"packages": map[string]any{
				"type": "packages",
				"pip":  []string{"requests==2.32.3"},
			},
		},
	}

	var env environmentAPIModel
	if err := c.Post(ctx, "/v1/environments", payload, &env); err != nil {
		t.Fatalf("create environment with packages: %v", err)
	}
	t.Logf("Created environment with packages: id=%s", env.ID)

	defer func() {
		c.Delete(ctx, fmt.Sprintf("/v1/environments/%s", env.ID))
		t.Logf("Cleaned up environment: id=%s", env.ID)
	}()

	if env.Config == nil {
		t.Fatal("config is nil")
	}
	t.Logf("Environment config: %v", env.Config)
}

// ---------- Agent CRUD ----------

func TestIntegrationAgent_CRUD(t *testing.T) {
	skipUnlessAcceptance(t)
	ctx := context.Background()
	c := testClient(t)

	// CREATE
	createPayload := map[string]any{
		"name":        uniqueName("agent"),
		"model":       "claude-sonnet-4-6",
		"description": "acceptance test agent",
		"system":      "You are a helpful test agent.",
	}

	var agent agentAPIModel
	if err := c.Post(ctx, "/v1/agents", createPayload, &agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Logf("Created agent: id=%s name=%s version=%d", agent.ID, agent.Name, agent.Version)

	if agent.ID == "" {
		t.Fatal("agent ID is empty")
	}

	// READ
	var readAgent agentAPIModel
	if err := c.Get(ctx, fmt.Sprintf("/v1/agents/%s", agent.ID), &readAgent); err != nil {
		t.Fatalf("read agent: %v", err)
	}
	if readAgent.Name != agent.Name {
		t.Errorf("name=%s, want %s", readAgent.Name, agent.Name)
	}
	t.Logf("Read agent: id=%s version=%d", readAgent.ID, readAgent.Version)

	// UPDATE (with version for optimistic concurrency)
	updatePayload := map[string]any{
		"name":        agent.Name + "-updated",
		"description": "updated agent",
		"model":       "claude-sonnet-4-6",
		"version":     readAgent.Version,
	}

	var updatedAgent agentAPIModel
	if err := c.Post(ctx, fmt.Sprintf("/v1/agents/%s", agent.ID), updatePayload, &updatedAgent); err != nil {
		t.Fatalf("update agent: %v", err)
	}
	if updatedAgent.Version <= readAgent.Version {
		t.Errorf("version should increment: got %d, prev %d", updatedAgent.Version, readAgent.Version)
	}
	t.Logf("Updated agent: id=%s version=%d desc=%s", updatedAgent.ID, updatedAgent.Version, updatedAgent.Description)

	// ARCHIVE (agents don't support DELETE)
	if err := c.Post(ctx, fmt.Sprintf("/v1/agents/%s/archive", agent.ID), map[string]any{}, nil); err != nil {
		t.Fatalf("archive agent: %v", err)
	}
	t.Logf("Archived agent: id=%s", agent.ID)

	// Verify archived
	var archivedAgent agentAPIModel
	if err := c.Get(ctx, fmt.Sprintf("/v1/agents/%s", agent.ID), &archivedAgent); err != nil {
		t.Fatalf("read archived agent: %v", err)
	}
	if archivedAgent.ArchivedAt == nil {
		t.Error("expected archived_at to be set")
	}
	t.Logf("Verified archived: archived_at=%v", *archivedAgent.ArchivedAt)
}

func TestIntegrationAgent_WithTools(t *testing.T) {
	skipUnlessAcceptance(t)
	ctx := context.Background()
	c := testClient(t)

	payload := map[string]any{
		"name":  uniqueName("agent-tools"),
		"model": "claude-sonnet-4-6",
		"tools": []map[string]any{
			{
				"type": "agent_toolset_20260401",
				"default_config": map[string]any{
					"enabled":           true,
					"permission_policy": map[string]any{"type": "always_allow"},
				},
				"configs": []map[string]any{
					{
						"name":              "bash",
						"enabled":           true,
						"permission_policy": map[string]any{"type": "always_allow"},
					},
					{
						"name":              "read",
						"enabled":           true,
						"permission_policy": map[string]any{"type": "always_allow"},
					},
				},
			},
		},
	}

	var agent agentAPIModel
	if err := c.Post(ctx, "/v1/agents", payload, &agent); err != nil {
		t.Fatalf("create agent with tools: %v", err)
	}
	t.Logf("Created agent with tools: id=%s", agent.ID)

	defer func() {
		c.Post(ctx, fmt.Sprintf("/v1/agents/%s/archive", agent.ID), map[string]any{}, nil)
		t.Logf("Cleaned up agent: id=%s", agent.ID)
	}()

	if len(agent.Tools) == 0 {
		t.Error("expected tools to be returned")
	} else {
		t.Logf("Agent has %d tool(s), first type=%v", len(agent.Tools), agent.Tools[0].Type)
	}
}

func TestIntegrationAgent_WithSkills(t *testing.T) {
	skipUnlessAcceptance(t)
	ctx := context.Background()
	c := testClient(t)

	payload := map[string]any{
		"name":  uniqueName("agent-skills"),
		"model": "claude-sonnet-4-6",
		"skills": []map[string]any{
			{
				"type":     "anthropic",
				"skill_id": "xlsx",
			},
		},
	}

	var agent agentAPIModel
	if err := c.Post(ctx, "/v1/agents", payload, &agent); err != nil {
		t.Fatalf("create agent with skills: %v", err)
	}
	t.Logf("Created agent with skills: id=%s", agent.ID)

	defer func() {
		c.Post(ctx, fmt.Sprintf("/v1/agents/%s/archive", agent.ID), map[string]any{}, nil)
		t.Logf("Cleaned up agent: id=%s", agent.ID)
	}()

	if len(agent.Skills) == 0 {
		t.Error("expected skills to be returned")
	} else {
		t.Logf("Agent has %d skill(s)", len(agent.Skills))
	}
}

// ---------- Vault CRUD ----------

func TestIntegrationVault_CRUD(t *testing.T) {
	skipUnlessAcceptance(t)
	ctx := context.Background()
	c := testClient(t)

	// CREATE
	var vault vaultAPIModel
	if err := c.Post(ctx, "/v1/vaults", map[string]any{
		"display_name": uniqueName("vault"),
	}, &vault); err != nil {
		t.Fatalf("create vault: %v", err)
	}
	t.Logf("Created vault: id=%s display_name=%s", vault.ID, vault.DisplayName)

	if vault.ID == "" {
		t.Fatal("vault ID is empty")
	}

	// READ
	var readVault vaultAPIModel
	if err := c.Get(ctx, fmt.Sprintf("/v1/vaults/%s", vault.ID), &readVault); err != nil {
		t.Fatalf("read vault: %v", err)
	}
	if readVault.DisplayName != vault.DisplayName {
		t.Errorf("display_name=%s, want %s", readVault.DisplayName, vault.DisplayName)
	}
	t.Logf("Read vault: id=%s", readVault.ID)

	// UPDATE
	var updatedVault vaultAPIModel
	if err := c.Post(ctx, fmt.Sprintf("/v1/vaults/%s", vault.ID), map[string]any{
		"display_name": vault.DisplayName + "-updated",
	}, &updatedVault); err != nil {
		t.Fatalf("update vault: %v", err)
	}
	if updatedVault.DisplayName != vault.DisplayName+"-updated" {
		t.Errorf("display_name=%s", updatedVault.DisplayName)
	}
	t.Logf("Updated vault: id=%s display_name=%s", updatedVault.ID, updatedVault.DisplayName)

	// DELETE
	if err := c.Delete(ctx, fmt.Sprintf("/v1/vaults/%s", vault.ID)); err != nil {
		t.Fatalf("delete vault: %v", err)
	}
	t.Logf("Deleted vault: id=%s", vault.ID)
}

// ---------- Full stack: env + agent + vault ----------

func TestIntegrationFullStack(t *testing.T) {
	skipUnlessAcceptance(t)
	ctx := context.Background()
	c := testClient(t)

	// 1. Create environment with packages
	var env environmentAPIModel
	if err := c.Post(ctx, "/v1/environments", map[string]any{
		"name":        uniqueName("full-env"),
		"description": "full stack test",
		"config": map[string]any{
			"type": "cloud",
			"networking": map[string]any{
				"type":              "limited",
				"allow_mcp_servers": true,
			},
		},
		"metadata": map[string]string{"test": "full-stack"},
	}, &env); err != nil {
		t.Fatalf("create environment: %v", err)
	}
	t.Logf("1. Environment created: id=%s", env.ID)
	defer func() {
		c.Delete(ctx, fmt.Sprintf("/v1/environments/%s", env.ID))
		t.Logf("Cleaned up environment %s", env.ID)
	}()

	// 2. Create agent with tools
	var agent agentAPIModel
	if err := c.Post(ctx, "/v1/agents", map[string]any{
		"name":        uniqueName("full-agent"),
		"model":       "claude-sonnet-4-6",
		"description": "full stack test agent",
		"system":      "You are a test agent. Respond briefly.",
		"tools": []map[string]any{
			{
				"type": "agent_toolset_20260401",
				"default_config": map[string]any{
					"enabled":           true,
					"permission_policy": map[string]any{"type": "always_allow"},
				},
			},
		},
		"metadata": map[string]string{"test": "full-stack"},
	}, &agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Logf("2. Agent created: id=%s version=%d", agent.ID, agent.Version)
	defer func() {
		c.Post(ctx, fmt.Sprintf("/v1/agents/%s/archive", agent.ID), map[string]any{}, nil)
		t.Logf("Cleaned up agent %s", agent.ID)
	}()

	// 3. Create vault
	var vault vaultAPIModel
	if err := c.Post(ctx, "/v1/vaults", map[string]any{
		"display_name": uniqueName("full-vault"),
	}, &vault); err != nil {
		t.Fatalf("create vault: %v", err)
	}
	t.Logf("3. Vault created: id=%s", vault.ID)
	defer func() {
		c.Delete(ctx, fmt.Sprintf("/v1/vaults/%s", vault.ID))
		t.Logf("Cleaned up vault %s", vault.ID)
	}()

	// 4. Update agent (version bump)
	var updatedAgent agentAPIModel
	var currentAgent agentAPIModel
	if err := c.Get(ctx, fmt.Sprintf("/v1/agents/%s", agent.ID), &currentAgent); err != nil {
		t.Fatalf("read agent for update: %v", err)
	}
	if err := c.Post(ctx, fmt.Sprintf("/v1/agents/%s", agent.ID), map[string]any{
		"name":        currentAgent.Name,
		"model":       "claude-sonnet-4-6",
		"description": "updated full stack test agent",
		"version":     currentAgent.Version,
	}, &updatedAgent); err != nil {
		t.Fatalf("update agent: %v", err)
	}
	t.Logf("4. Agent updated: id=%s version=%d->%d", agent.ID, currentAgent.Version, updatedAgent.Version)

	t.Log("Full stack acceptance test completed successfully!")
}

// ---------- Flatten roundtrip: API -> TF state -> verify ----------

func TestIntegrationAgent_FlattenRoundtrip(t *testing.T) {
	skipUnlessAcceptance(t)
	ctx := context.Background()
	c := testClient(t)

	payload := map[string]any{
		"name":        uniqueName("rt-agent"),
		"model":       map[string]any{"id": "claude-sonnet-4-6", "speed": "standard"},
		"description": "roundtrip test",
		"system":      "Test system prompt.",
		"metadata":    map[string]string{"key1": "val1"},
		"tools": []map[string]any{
			{
				"type": "agent_toolset_20260401",
				"default_config": map[string]any{
					"enabled":           true,
					"permission_policy": map[string]any{"type": "always_allow"},
				},
				"configs": []map[string]any{
					{"name": "bash", "enabled": true, "permission_policy": map[string]any{"type": "always_allow"}},
				},
			},
		},
		"skills": []map[string]any{
			{"type": "anthropic", "skill_id": "xlsx"},
		},
	}

	var agent agentAPIModel
	if err := c.Post(ctx, "/v1/agents", payload, &agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Logf("Created agent: id=%s", agent.ID)
	defer func() {
		c.Post(ctx, fmt.Sprintf("/v1/agents/%s/archive", agent.ID), map[string]any{}, nil)
	}()

	// Flatten to TF state
	state, diags := flattenAgentState(ctx, agent)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}

	if state.ID.ValueString() != agent.ID {
		t.Errorf("id mismatch")
	}
	if state.Name.ValueString() != agent.Name {
		t.Errorf("name=%s, want %s", state.Name.ValueString(), agent.Name)
	}
	if state.Description.ValueString() != "roundtrip test" {
		t.Errorf("description=%s", state.Description.ValueString())
	}
	if state.System.ValueString() != "Test system prompt." {
		t.Errorf("system=%s", state.System.ValueString())
	}
	if state.ModelID.IsNull() {
		t.Error("model_id is null")
	}
	if state.Tools.IsNull() {
		t.Error("tools is null")
	} else {
		t.Logf("Tools count: %d", len(state.Tools.Elements()))
	}
	if state.Skills.IsNull() {
		t.Error("skills is null")
	} else {
		t.Logf("Skills count: %d", len(state.Skills.Elements()))
	}
	if state.Metadata.IsNull() {
		t.Error("metadata is null")
	}

	t.Log("Flatten roundtrip verified successfully")
}
