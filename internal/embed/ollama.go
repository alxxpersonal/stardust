// Package embed produces vector embeddings via a local Ollama server. An
// Available check lets retrieval and indexing degrade to FTS5-only when Ollama
// or the configured model is absent, so Stardust never hard-fails on it.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client talks to an Ollama server's embedding endpoint.
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// New returns a Client for the given Ollama base URL and embedding model.
func New(baseURL, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

// Model returns the configured embedding model name.
func (c *Client) Model() string { return c.model }

// Available reports whether the Ollama server is reachable and the model is
// pulled. A short timeout keeps the check cheap on the index/query hot path.
func (c *Client) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false
	}
	for _, m := range body.Models {
		if m.Name == c.model || strings.HasPrefix(m.Name, c.model+":") {
			return true
		}
	}
	return false
}

// Embed returns one vector per input text via Ollama's batch /api/embed endpoint.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(map[string]any{"model": c.model, "input": texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed: ollama returned %s", resp.Status)
	}
	var body struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if len(body.Embeddings) != len(texts) {
		return nil, fmt.Errorf("embed: got %d vectors for %d inputs", len(body.Embeddings), len(texts))
	}
	return body.Embeddings, nil
}
