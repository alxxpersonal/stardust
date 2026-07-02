// Package rerank optionally re-scores hybrid search results with a cross-encoder
// reranker served over an HTTP endpoint (llama.cpp /v1/rerank, or any
// Jina/Cohere-compatible server). It degrades gracefully: with no endpoint
// configured, or one that is unreachable or errors, Rerank returns the input
// unchanged, exactly like embeddings fall back to FTS-only when Ollama is down.
package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/alxxpersonal/stardust/internal/index"
)

// Client calls a cross-encoder reranker endpoint.
type Client struct {
	url   string
	path  string
	model string
	http  *http.Client
}

// New returns a reranker client for the given endpoint URL (empty = disabled)
// and optional model name, targeting the /v1/rerank contract.
func New(url, model string) *Client {
	return newClient(url, OpenAIRerankPath, model)
}

// newClient returns a reranker client for a base URL, a rerank endpoint path
// (empty defaults to /v1/rerank), and an optional model name. Discovery uses it
// to build a client for a discovered runtime whose rerank path may differ from
// the /v1/rerank default (for example Ollama's /api/rerank seam).
func newClient(url, path, model string) *Client {
	if path == "" {
		path = OpenAIRerankPath
	}
	return &Client{
		url:   strings.TrimRight(url, "/"),
		path:  path,
		model: model,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Enabled reports whether a reranker endpoint is configured.
func (c *Client) Enabled() bool { return c.url != "" }

// Rerank re-orders hits by cross-encoder relevance to the query. It NEVER fails
// the caller: with no endpoint, fewer than two hits, or any transport/decode
// error, it returns hits unchanged.
func (c *Client) Rerank(ctx context.Context, query string, hits []index.Hit) []index.Hit {
	if c.url == "" || len(hits) < 2 {
		return hits
	}

	docs := make([]string, len(hits))
	for i, h := range hits {
		docs[i] = rerankText(h)
	}
	payload := map[string]any{"query": query, "documents": docs}
	if c.model != "" {
		payload["model"] = c.model
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return hits
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+c.path, bytes.NewReader(body))
	if err != nil {
		return hits
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return hits
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return hits
	}

	var out struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Results) == 0 {
		return hits
	}

	reranked := make([]index.Hit, 0, len(hits))
	for _, r := range out.Results {
		if r.Index < 0 || r.Index >= len(hits) {
			continue
		}
		h := hits[r.Index]
		h.Score = r.RelevanceScore
		reranked = append(reranked, h)
	}
	if len(reranked) == 0 {
		return hits
	}
	sort.SliceStable(reranked, func(i, j int) bool { return reranked[i].Score > reranked[j].Score })
	return reranked
}

// rerankText is the document text handed to the cross-encoder for a hit.
func rerankText(h index.Hit) string {
	parts := make([]string, 0, 3)
	for _, p := range []string{h.Title, h.Heading, h.Snippet} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " - ")
}
