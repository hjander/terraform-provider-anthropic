package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ClientConfig struct {
	BaseURL           string
	APIKey            string
	AnthropicVersion  string
	ManagedAgentsBeta string
}

type Client struct {
	baseURL           string
	apiKey            string
	anthropicVersion  string
	managedAgentsBeta string
	httpClient        *http.Client
}

type apiErrorEnvelope struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Error     struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// NotFoundError is returned by Client methods when the API responds with 404,
// allowing Read methods to distinguish "resource deleted" from real failures.
type NotFoundError struct {
	StatusCode int
	Message    string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found (status %d): %s", e.StatusCode, e.Message)
}

func NewClient(cfg ClientConfig) *Client {
	return &Client{
		baseURL:           strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:            cfg.APIKey,
		anthropicVersion:  cfg.AnthropicVersion,
		managedAgentsBeta: cfg.ManagedAgentsBeta,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) Post(ctx context.Context, path string, payload any, out any) error {
	return c.do(ctx, http.MethodPost, path, payload, out)
}

func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(payload); err != nil {
			return err
		}
		body = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("content-type", "application/json")
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", c.anthropicVersion)
	req.Header.Set("anthropic-beta", c.managedAgentsBeta)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiErrorEnvelope
		msg := string(respBytes)
		if json.Unmarshal(respBytes, &apiErr) == nil && apiErr.Error.Message != "" {
			msg = fmt.Sprintf("type=%s request_id=%s message=%s", apiErr.Error.Type, apiErr.RequestID, apiErr.Error.Message)
		}
		if resp.StatusCode == http.StatusNotFound {
			return &NotFoundError{StatusCode: resp.StatusCode, Message: msg}
		}
		return fmt.Errorf("anthropic api error: status=%d %s", resp.StatusCode, msg)
	}

	if out == nil || len(respBytes) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("unmarshal response: %w; body=%s", err, string(respBytes))
	}
	return nil
}
