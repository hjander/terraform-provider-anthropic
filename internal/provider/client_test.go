package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Get(t *testing.T) {
	want := map[string]string{"id": "env_123", "name": "test"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/environments/env_123" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing api key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Error("missing anthropic-version header")
		}
		if r.Header.Get("anthropic-beta") != "managed-agents-2026-04-01" {
			t.Error("missing anthropic-beta header")
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{
		BaseURL:           srv.URL,
		APIKey:            "test-key",
		AnthropicVersion:  "2023-06-01",
		ManagedAgentsBeta: "managed-agents-2026-04-01",
	})

	var got map[string]string
	err := c.Get(context.Background(), "/v1/environments/env_123", &got)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got["id"] != "env_123" {
		t.Errorf("got id=%q, want env_123", got["id"])
	}
}

func TestClient_Post(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("content-type"); ct != "application/json" {
			t.Errorf("content-type=%q", ct)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "new-env" {
			t.Errorf("body name=%q", body["name"])
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "env_456", "name": "new-env"})
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})

	var got map[string]string
	err := c.Post(context.Background(), "/v1/environments", map[string]string{"name": "new-env"}, &got)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if got["id"] != "env_456" {
		t.Errorf("got id=%q", got["id"])
	}
}

func TestClient_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	err := c.Delete(context.Background(), "/v1/environments/env_123")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestClient_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"type":       "error",
			"request_id": "req_abc",
			"error": map[string]string{
				"type":    "invalid_request_error",
				"message": "name is required",
			},
		})
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	err := c.Get(context.Background(), "/v1/agents/bad", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got == "" {
		t.Error("empty error message")
	}
}

func TestClient_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	var out map[string]string
	err := c.Get(context.Background(), "/v1/something", &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil output, got %v", out)
	}
}
