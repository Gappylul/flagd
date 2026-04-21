// Package sdk provides a lightweight client for flagd.
//
// Usage:
//
//	client := sdk.New("https://flagd.gappy.hu")
//
//	if client.IsEnabled(ctx, "dark-mode") {
//	    // feature is on
//	}
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Flag is the flag object returned by flagd.
type Flag struct {
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Client talks to a flagd server.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option configures a client.
type Option func(*Client)

// WithTimeout sets a custom HTTP timeout (default: 2s).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// New creates a flagd client pointing at baseURL (e.g. "https://flagd.your-site.com").
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 2 * time.Second},
	}

	for _, o := range opts {
		o(c)
	}

	return c
}

// IsEnabled returns whether the named flag is enabled.
// Returns false on any error - a dead flag server never breaks your app.
func (c *Client) IsEnabled(ctx context.Context, name string) bool {
	f, err := c.Get(ctx, name)
	if err != nil {
		return false
	}
	return f.Enabled
}

// Get fetches a flag by name. Returns an error if the flag doesn't exist
// or the server is unreachable.
func (c *Client) Get(ctx context.Context, name string) (Flag, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/flags/%s", c.baseURL, name), nil)
	if err != nil {
		return Flag{}, fmt.Errorf("flagd: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Flag{}, fmt.Errorf("flagd: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Flag{}, fmt.Errorf("flagd: flag %q not found", name)
	}
	if resp.StatusCode != http.StatusOK {
		return Flag{}, fmt.Errorf("flagd: unexpected status %d", resp.StatusCode)
	}

	var f Flag
	if err := json.NewDecoder(resp.Body).Decode(&f); err != nil {
		return Flag{}, fmt.Errorf("flagd: decode response: %w", err)
	}
	return f, nil
}

// List returns all flags. Returns an empty slice on error.
func (c *Client) List(ctx context.Context) ([]Flag, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/flags", c.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("flagd: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("flagd: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flagd: unexpected status %d", resp.StatusCode)
	}

	var flags []Flag
	if err := json.NewDecoder(resp.Body).Decode(&flags); err != nil {
		return nil, fmt.Errorf("flagd: decode response: %w", err)
	}
	return flags, nil
}
