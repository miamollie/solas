package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// TagsResponse is Ollama /api/tags response.
type TagsResponse struct {
	Models []TagModel `json:"models"`
}

// TagModel is a model listed by Ollama.
type TagModel struct {
	Name string `json:"name"`
}

// ListModels fetches model tags from Ollama.
func (c *Client) ListModels(ctx context.Context) ([]TagModel, error) {
	u := c.baseURL.JoinPath("/api/tags")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	var tags TagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	return tags.Models, nil
}
