package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxResponseBytes = 10 << 20 // 10 MB
	maxRetries       = 3
	defaultTimeout   = 60 * time.Second
	userAgentPrefix  = "terraform-provider-anthropic"
)

type ClientConfig struct {
	BaseURL           string
	APIKey            string
	AnthropicVersion  string
	ManagedAgentsBeta string
	Timeout           time.Duration
	UserAgent         string
}

type Client struct {
	baseURL           string
	apiKey            string
	anthropicVersion  string
	managedAgentsBeta string
	userAgent         string
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
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = userAgentPrefix + "/" + version
	}
	return &Client{
		baseURL:           strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:            cfg.APIKey,
		anthropicVersion:  cfg.AnthropicVersion,
		managedAgentsBeta: cfg.ManagedAgentsBeta,
		userAgent:         ua,
		httpClient: &http.Client{
			Timeout: timeout,
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
	var bodyBytes []byte
	if payload != nil {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(payload); err != nil {
			return err
		}
		bodyBytes = buf.Bytes()
	}

	var lastErr error
	var retryAfter time.Duration
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 500 * time.Millisecond
			if retryAfter > 0 {
				backoff = retryAfter
				retryAfter = 0
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		var body io.Reader
		if bodyBytes != nil {
			body = bytes.NewReader(bodyBytes)
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
		req.Header.Set("user-agent", c.userAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out == nil || len(respBytes) == 0 {
				return nil
			}
			if err := json.Unmarshal(respBytes, out); err != nil {
				if int64(len(respBytes)) >= maxResponseBytes {
					return fmt.Errorf("unmarshal response (possibly truncated at %d bytes): %w", maxResponseBytes, err)
				}
				return fmt.Errorf("unmarshal response: %w; body=%s", err, string(respBytes))
			}
			return nil
		}

		var apiErr apiErrorEnvelope
		msg := string(respBytes)
		if json.Unmarshal(respBytes, &apiErr) == nil && apiErr.Error.Message != "" {
			msg = fmt.Sprintf("type=%s request_id=%s message=%s", apiErr.Error.Type, apiErr.RequestID, apiErr.Error.Message)
		}

		if resp.StatusCode == http.StatusNotFound {
			return &NotFoundError{StatusCode: resp.StatusCode, Message: msg}
		}

		// Retry on 429 (rate limit) and 5xx (server errors); all other errors are terminal.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if resp.StatusCode == http.StatusTooManyRequests {
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if secs, parseErr := strconv.Atoi(ra); parseErr == nil && secs > 0 {
						retryAfter = time.Duration(secs) * time.Second
					}
				}
			}
			lastErr = fmt.Errorf("anthropic api error: status=%d %s", resp.StatusCode, msg)
			continue
		}

		return fmt.Errorf("anthropic api error: status=%d %s", resp.StatusCode, msg)
	}
	return fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}
