package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Client wraps Ollama HTTP interactions.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// NewClient creates an Ollama client.
func NewClient(baseURL string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse ollama base url: %w", err)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: u, httpClient: httpClient}, nil
}

// IsReachable checks whether Ollama is reachable.
func (c *Client) IsReachable(ctx context.Context) error {
	u := c.baseURL.JoinPath("/api/tags")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return nil
}
