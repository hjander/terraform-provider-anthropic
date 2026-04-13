package provider

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func skipUnlessTerraformAcc(t *testing.T) {
	t.Helper()
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set; skipping Terraform acceptance test")
	}
}

func testAccUniqueName(prefix string) string {
	return fmt.Sprintf("%s-tfacc-%d", prefix, time.Now().UnixMilli())
}

func testAccCheckEnvironmentDestroy(s *terraform.State) error {
	c := NewClient(ClientConfig{
		BaseURL:           "https://api.anthropic.com",
		APIKey:            os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicVersion:  defaultAnthropicVer,
		ManagedAgentsBeta: defaultManagedAgents,
	})
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "anthropic_managed_environment" {
			continue
		}
		var env environmentAPIModel
		err := c.Get(context.Background(), fmt.Sprintf("/v1/environments/%s", rs.Primary.ID), &env)
		if err == nil && env.ArchivedAt == nil {
			return fmt.Errorf("environment %s still exists and is not archived", rs.Primary.ID)
		}
	}
	return nil
}

func testAccCheckAgentDestroy(s *terraform.State) error {
	c := NewClient(ClientConfig{
		BaseURL:           "https://api.anthropic.com",
		APIKey:            os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicVersion:  defaultAnthropicVer,
		ManagedAgentsBeta: defaultManagedAgents,
	})
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "anthropic_managed_agent" {
			continue
		}
		var agent agentAPIModel
		err := c.Get(context.Background(), fmt.Sprintf("/v1/agents/%s", rs.Primary.ID), &agent)
		if err == nil && agent.ArchivedAt == nil {
			return fmt.Errorf("agent %s still exists and is not archived", rs.Primary.ID)
		}
	}
	return nil
}

func testAccCheckVaultDestroy(s *terraform.State) error {
	c := NewClient(ClientConfig{
		BaseURL:           "https://api.anthropic.com",
		APIKey:            os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicVersion:  defaultAnthropicVer,
		ManagedAgentsBeta: defaultManagedAgents,
	})
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "anthropic_managed_vault" {
			continue
		}
		var vault vaultAPIModel
		err := c.Get(context.Background(), fmt.Sprintf("/v1/vaults/%s", rs.Primary.ID), &vault)
		if err == nil && vault.ArchivedAt == nil {
			return fmt.Errorf("vault %s still exists and is not archived", rs.Primary.ID)
		}
	}
	return nil
}

// Terraform acceptance tests: these go through the full provider plan/apply lifecycle
// via terraform-plugin-testing, unlike the TestIntegration* tests which call the HTTP
// client directly.

// ---------- Environment ----------

func TestAccEnvironment_basic(t *testing.T) {
	skipUnlessTerraformAcc(t)
	name := testAccUniqueName("env")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEnvironmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "anthropic_managed_environment" "test" {
  name = %q
  config = {
    type = "cloud"
    networking = {
      type = "unrestricted"
    }
  }
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("anthropic_managed_environment.test", "id"),
					resource.TestCheckResourceAttr("anthropic_managed_environment.test", "name", name),
					resource.TestCheckResourceAttr("anthropic_managed_environment.test", "config.type", "cloud"),
					resource.TestCheckResourceAttr("anthropic_managed_environment.test", "config.networking.type", "unrestricted"),
				),
			},
			{
				ResourceName:      "anthropic_managed_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: fmt.Sprintf(`
resource "anthropic_managed_environment" "test" {
  name        = %q
  description = "updated"
  config = {
    type = "cloud"
    networking = {
      type                   = "limited"
      allow_package_managers = true
    }
  }
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("anthropic_managed_environment.test", "description", "updated"),
					resource.TestCheckResourceAttr("anthropic_managed_environment.test", "config.networking.type", "limited"),
				),
			},
		},
	})
}

// ---------- Agent ----------

func TestAccAgent_basic(t *testing.T) {
	skipUnlessTerraformAcc(t)
	name := testAccUniqueName("agent")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAgentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "anthropic_managed_agent" "test" {
  name     = %q
  model_id = "claude-sonnet-4-6"
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("anthropic_managed_agent.test", "id"),
					resource.TestCheckResourceAttr("anthropic_managed_agent.test", "name", name),
					resource.TestCheckResourceAttr("anthropic_managed_agent.test", "model_id", "claude-sonnet-4-6"),
					resource.TestCheckResourceAttrSet("anthropic_managed_agent.test", "version"),
				),
			},
			{
				ResourceName:      "anthropic_managed_agent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: fmt.Sprintf(`
resource "anthropic_managed_agent" "test" {
  name        = %q
  model_id    = "claude-sonnet-4-6"
  description = "updated agent"
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("anthropic_managed_agent.test", "description", "updated agent"),
				),
			},
		},
	})
}

func TestAccAgent_withTools(t *testing.T) {
	skipUnlessTerraformAcc(t)
	name := testAccUniqueName("agent-tools")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAgentDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "anthropic_managed_agent" "test" {
  name     = %q
  model_id = "claude-sonnet-4-6"

  tools = [{
    type = "agent_toolset_20260401"
    default_config = {
      enabled          = true
      permission_policy = "always_allow"
    }
    configs = [{
      name             = "bash"
      enabled          = true
      permission_policy = "always_allow"
    }]
  }]
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("anthropic_managed_agent.test", "id"),
					resource.TestCheckResourceAttr("anthropic_managed_agent.test", "tools.#", "1"),
					resource.TestCheckResourceAttr("anthropic_managed_agent.test", "tools.0.type", "agent_toolset_20260401"),
				),
			},
		},
	})
}

// ---------- Vault ----------

func testAccCheckVaultCredentialDestroy(s *terraform.State) error {
	c := NewClient(ClientConfig{
		BaseURL:           "https://api.anthropic.com",
		APIKey:            os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicVersion:  defaultAnthropicVer,
		ManagedAgentsBeta: defaultManagedAgents,
	})
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "anthropic_managed_vault_credential" {
			continue
		}
		vaultID := rs.Primary.Attributes["vault_id"]
		var cred credentialAPIModel
		err := c.Get(context.Background(), fmt.Sprintf("/v1/vaults/%s/credentials/%s", vaultID, rs.Primary.ID), &cred)
		if err == nil && cred.ArchivedAt == nil {
			return fmt.Errorf("credential %s still exists and is not archived", rs.Primary.ID)
		}
	}
	return nil
}

func TestAccVault_basic(t *testing.T) {
	skipUnlessTerraformAcc(t)
	name := testAccUniqueName("vault")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVaultDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "anthropic_managed_vault" "test" {
  display_name = %q
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("anthropic_managed_vault.test", "id"),
					resource.TestCheckResourceAttr("anthropic_managed_vault.test", "display_name", name),
				),
			},
			{
				ResourceName:      "anthropic_managed_vault.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: fmt.Sprintf(`
resource "anthropic_managed_vault" "test" {
  display_name = "%s-updated"
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("anthropic_managed_vault.test", "display_name", name+"-updated"),
				),
			},
		},
	})
}

// ---------- Vault Credential ----------

func TestAccVaultCredential_basic(t *testing.T) {
	skipUnlessTerraformAcc(t)
	vaultName := testAccUniqueName("vault")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVaultCredentialDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "anthropic" {}

resource "anthropic_managed_vault" "test" {
  display_name = %q
}

resource "anthropic_managed_vault_credential" "test" {
  vault_id     = anthropic_managed_vault.test.id
  display_name = "test-credential"
  auth = {
    type           = "mcp_oauth"
    mcp_server_url = "https://mcp.example.com/server"
    access_token   = "test-token-123"
  }
}
`, vaultName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("anthropic_managed_vault_credential.test", "id"),
					resource.TestCheckResourceAttr("anthropic_managed_vault_credential.test", "display_name", "test-credential"),
					resource.TestCheckResourceAttr("anthropic_managed_vault_credential.test", "credential_type", "mcp_oauth"),
				),
			},
		},
	})
}
