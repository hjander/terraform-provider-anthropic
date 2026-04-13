#!/usr/bin/env python3
"""Verify Terraform-created resources via direct API calls (not the provider)."""

import json
import os
import subprocess
import sys

import httpx

BASE = "https://api.anthropic.com"
HEADERS = {
    "x-api-key": os.environ["ANTHROPIC_API_KEY"],
    "anthropic-version": "2023-06-01",
    "anthropic-beta": "managed-agents-2026-04-01",
}


def tf_outputs() -> dict[str, str]:
    raw = subprocess.check_output(
        ["terraform", "output", "-json"],
        text=True,
    )
    return {k: v["value"] for k, v in json.loads(raw).items()}


def get(path: str) -> dict:
    r = httpx.get(f"{BASE}{path}", headers=HEADERS, timeout=15)
    r.raise_for_status()
    return r.json()


def print_section(title: str):
    print(f"\n{'=' * 60}")
    print(f"  {title}")
    print(f"{'=' * 60}")


def print_row(label: str, value):
    print(f"  {label:<28} {value}")


def main():
    outputs = tf_outputs()

    env_ids = {
        "sandbox": outputs["sandbox_env_id"],
        "production": outputs["production_env_id"],
    }
    agent_ids = {
        "minimal": outputs["minimal_agent_id"],
        "tooled": outputs["tooled_agent_id"],
        "custom_tool": outputs["custom_tool_agent_id"],
    }
    vault_id = outputs["vault_id"]

    print_section("ENVIRONMENTS")
    for name, eid in env_ids.items():
        env = get(f"/v1/environments/{eid}")
        print(f"\n  [{name}]")
        print_row("id", env["id"])
        print_row("name", env["name"])
        print_row("description", env.get("description", ""))
        cfg = env.get("config", {})
        net = cfg.get("networking", {})
        print_row("networking.type", net.get("type"))
        if net.get("type") == "limited":
            print_row("allowed_hosts", net.get("allowed_hosts", []))
            print_row("allow_pkg_mgrs", net.get("allow_package_managers"))
        pkgs = cfg.get("packages")
        if pkgs:
            for mgr in ("pip", "npm", "apt"):
                items = pkgs.get(mgr, [])
                if items:
                    print_row(f"packages.{mgr}", items)
        print_row("metadata", env.get("metadata", {}))

    print_section("AGENTS")
    for name, aid in agent_ids.items():
        agent = get(f"/v1/agents/{aid}")
        print(f"\n  [{name}]")
        print_row("id", agent["id"])
        print_row("name", agent["name"])
        print_row("description", agent.get("description", ""))
        model = agent.get("model", {})
        print_row("model.id", model.get("id"))
        print_row("model.speed", model.get("speed"))
        print_row("version", agent.get("version"))
        tools = agent.get("tools", [])
        if tools:
            print_row("tools_count", len(tools))
            for i, t in enumerate(tools):
                print_row(f"  tool[{i}].type", t.get("type"))
                if t.get("name"):
                    print_row(f"  tool[{i}].name", t["name"])

    print_section("VAULT")
    vault = get(f"/v1/vaults/{vault_id}")
    print_row("id", vault["id"])
    print_row("display_name", vault.get("display_name"))
    print_row("metadata", vault.get("metadata", {}))

    print(f"\n{'=' * 60}")
    print("  VERIFICATION COMPLETE")
    print(f"  Environments: {len(env_ids)}, Agents: {len(agent_ids)}, "
          f"Vault: 1")
    print(f"{'=' * 60}\n")


if __name__ == "__main__":
    main()
