package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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
		if ua := r.Header.Get("user-agent"); ua == "" {
			t.Error("missing user-agent header")
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

func TestClient_NotFoundError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"type":       "error",
			"request_id": "req_xyz",
			"error": map[string]string{
				"type":    "not_found_error",
				"message": "Agent not found",
			},
		})
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	err := c.Get(context.Background(), "/v1/agents/missing", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var nfe *NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
	if nfe.StatusCode != 404 {
		t.Errorf("status=%d, want 404", nfe.StatusCode)
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

func TestClient_ServerError_Retries(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"internal error"}}`))
			return
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "ok"})
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	var out map[string]string
	err := c.Get(context.Background(), "/v1/test", &out)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if out["id"] != "ok" {
		t.Errorf("got %v", out)
	}
	if n := attempts.Load(); n != 3 {
		t.Errorf("expected 3 attempts (2 retries + success), got %d", n)
	}
}

func TestClient_RateLimit_Retries(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`))
			return
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "ok"})
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	var out map[string]string
	err := c.Get(context.Background(), "/v1/test", &out)
	if err != nil {
		t.Fatalf("expected success after rate limit retry, got: %v", err)
	}
	if n := attempts.Load(); n != 2 {
		t.Errorf("expected 2 attempts, got %d", n)
	}
}

func TestClient_ForbiddenError_NoRetry(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"type":       "error",
			"request_id": "req_f",
			"error": map[string]string{
				"type":    "permission_error",
				"message": "not authorized",
			},
		})
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	err := c.Get(context.Background(), "/v1/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if n := attempts.Load(); n != 1 {
		t.Errorf("403 should not retry, got %d attempts", n)
	}
}

func TestClient_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	var out map[string]string
	err := c.Get(context.Background(), "/v1/test", &out)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestClient_UserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("user-agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	c.Get(context.Background(), "/v1/test", nil)
	if gotUA == "" {
		t.Error("expected user-agent header")
	}
}

func TestClient_CustomUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("user-agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b", UserAgent: "custom-agent/1.0"})
	c.Get(context.Background(), "/v1/test", nil)
	if gotUA != "custom-agent/1.0" {
		t.Errorf("user-agent=%q, want custom-agent/1.0", gotUA)
	}
}

func TestClient_RateLimit_RespectsRetryAfter(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`))
			return
		}
		w.Header().Set("content-type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "ok"})
	}))
	defer srv.Close()

	c := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "k", AnthropicVersion: "v", ManagedAgentsBeta: "b"})
	var out map[string]string
	err := c.Get(context.Background(), "/v1/test", &out)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if n := attempts.Load(); n != 2 {
		t.Errorf("expected 2 attempts, got %d", n)
	}
}
