// Package sdk is a typed Go client for the Stardust HTTP/JSON API. It decouples
// callers from the server internals: the wire types here mirror the API's JSON
// (docs/openapi.yaml) and are intentionally independent of the internal packages.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client talks to a Stardust HTTP API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client for the given base URL (e.g. http://127.0.0.1:7777).
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// --- Wire types (mirror docs/openapi.yaml) ---

type Hit struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Heading string  `json:"heading"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

type QueryResult struct {
	Query string `json:"query"`
	Mode  string `json:"mode"`
	Hits  []Hit  `json:"hits"`
}

type Note struct {
	Path  string   `json:"path"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
	Links []string `json:"links"`
	Body  string   `json:"body"`
}

type Status struct {
	Root        string `json:"root"`
	Notes       int    `json:"notes"`
	Chunks      int    `json:"chunks"`
	LastIndexed string `json:"last_indexed_sha"`
	EmbedModel  string `json:"embed_model"`
	Vectors     bool   `json:"vectors"`
	Reranker    bool   `json:"reranker"`
}

type BrokenLink struct {
	From   string `json:"from"`
	Target string `json:"target"`
}

type GraphReport struct {
	Notes   int          `json:"notes"`
	Links   int          `json:"links"`
	Orphans []string     `json:"orphans"`
	Broken  []BrokenLink `json:"broken"`
}

type BundleItem struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

type BundleResult struct {
	Task     string       `json:"task"`
	Items    []BundleItem `json:"items"`
	Markdown string       `json:"markdown"`
	Tokens   int          `json:"tokens_estimate"`
}

type DigestResult struct {
	Since    string `json:"since"`
	Head     string `json:"head"`
	Changed  int    `json:"changed"`
	Markdown string `json:"markdown"`
}

type IndexStats struct {
	Indexed int  `json:"indexed"`
	Skipped int  `json:"skipped"`
	Deleted int  `json:"deleted"`
	Vectors bool `json:"vectors"`
}

// --- Methods ---

// Query runs a hybrid search.
func (c *Client) Query(ctx context.Context, q string, limit int) (QueryResult, error) {
	var out QueryResult
	err := c.do(ctx, http.MethodGet, "/query", url.Values{"q": {q}, "limit": {strconv.Itoa(limit)}}, &out)
	return out, err
}

// GetNote reads a parsed note by vault-relative path.
func (c *Client) GetNote(ctx context.Context, path string) (Note, error) {
	var out Note
	err := c.do(ctx, http.MethodGet, "/note", url.Values{"path": {path}}, &out)
	return out, err
}

// Status reports index health.
func (c *Client) Status(ctx context.Context) (Status, error) {
	var out Status
	err := c.do(ctx, http.MethodGet, "/status", nil, &out)
	return out, err
}

// Graph reports the link graph.
func (c *Client) Graph(ctx context.Context) (GraphReport, error) {
	var out GraphReport
	err := c.do(ctx, http.MethodGet, "/graph", nil, &out)
	return out, err
}

// Bundle assembles a task-scoped context bundle.
func (c *Client) Bundle(ctx context.Context, task string, budget int) (BundleResult, error) {
	var out BundleResult
	err := c.do(ctx, http.MethodGet, "/bundle", url.Values{"task": {task}, "budget": {strconv.Itoa(budget)}}, &out)
	return out, err
}

// Digest summarizes recent activity.
func (c *Client) Digest(ctx context.Context, since string, advance bool) (DigestResult, error) {
	var out DigestResult
	v := url.Values{}
	if since != "" {
		v.Set("since", since)
	}
	if advance {
		v.Set("advance", "true")
	}
	err := c.do(ctx, http.MethodGet, "/digest", v, &out)
	return out, err
}

// Index incrementally indexes the vault.
func (c *Client) Index(ctx context.Context, since string) (IndexStats, error) {
	var out IndexStats
	v := url.Values{}
	if since != "" {
		v.Set("since", since)
	}
	err := c.do(ctx, http.MethodPost, "/index", v, &out)
	return out, err
}

// Rebuild nukes the cache and reindexes.
func (c *Client) Rebuild(ctx context.Context) (IndexStats, error) {
	var out IndexStats
	err := c.do(ctx, http.MethodPost, "/rebuild", nil, &out)
	return out, err
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, out any) error {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(body)))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
	}
	return nil
}
