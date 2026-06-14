// Package sdk is a typed Go client for the Stardust HTTP/JSON API. It decouples
// callers from the server internals: the wire types here mirror the API's JSON
// (docs/openapi.yaml) and are intentionally independent of the internal packages.
package sdk

import (
	"bytes"
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

type LinkTarget struct {
	Link string `json:"link"`
	Path string `json:"path"`
}

type Note struct {
	Path        string       `json:"path"`
	Title       string       `json:"title"`
	Tags        []string     `json:"tags"`
	Links       []string     `json:"links"`
	LinkTargets []LinkTarget `json:"link_targets"`
	Body        string       `json:"body"`
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

type PageRankEntry struct {
	Path  string  `json:"path"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

type GraphReport struct {
	Notes    int             `json:"notes"`
	Links    int             `json:"links"`
	Orphans  []string        `json:"orphans"`
	Broken   []BrokenLink    `json:"broken"`
	PageRank []PageRankEntry `json:"pagerank"`
}

type Mount struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Target string   `json:"target"`
	Args   []string `json:"args,omitempty"`
	Tool   string   `json:"tool"`
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

// Field is one typed column of a collection schema.
type Field struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Enum     []string `json:"enum,omitempty"`
	Default  any      `json:"default,omitempty"`
}

// CollectionInfo describes a collection: its name, vault folder, description,
// schema fields, and live record count.
type CollectionInfo struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
	Records     int     `json:"records"`
}

// Predicate is a single frontmatter filter for ListRecords. Op is one of eq, ne,
// gt, gte, lt, lte, contains.
type Predicate struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// Record is a single note in a collection: its path, title, frontmatter columns,
// and (when read whole) its markdown body.
type Record struct {
	Path        string         `json:"path"`
	Title       string         `json:"title"`
	Frontmatter map[string]any `json:"frontmatter"`
	Body        string         `json:"body"`
}

// RecordList is a page of records for a collection, with the resolved folder.
type RecordList struct {
	Collection string   `json:"collection"`
	Folder     string   `json:"folder"`
	Records    []Record `json:"records"`
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

// Mounts lists the configured external-source mounts.
func (c *Client) Mounts(ctx context.Context) ([]Mount, error) {
	var out []Mount
	err := c.do(ctx, http.MethodGet, "/mounts", nil, &out)
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

// ListCollections lists every configured collection with a live record count.
func (c *Client) ListCollections(ctx context.Context) ([]CollectionInfo, error) {
	var out []CollectionInfo
	err := c.do(ctx, http.MethodGet, "/collections", nil, &out)
	return out, err
}

// GetCollection returns a single collection by name.
func (c *Client) GetCollection(ctx context.Context, name string) (CollectionInfo, error) {
	var out CollectionInfo
	err := c.do(ctx, http.MethodGet, "/collection", url.Values{"name": {name}}, &out)
	return out, err
}

// ListRecords lists records in a collection, filtered by frontmatter predicates
// and ordered by sort (a frontmatter field, or "path" / "updated_at", with an
// optional leading "-" for descending). A non-positive limit means no limit.
func (c *Client) ListRecords(ctx context.Context, collection string, filter []Predicate, sort string, limit, offset int) (RecordList, error) {
	var out RecordList
	v := url.Values{"collection": {collection}}
	for _, p := range filter {
		v.Add("where", p.Field+":"+p.Op+":"+p.Value)
	}
	if sort != "" {
		v.Set("sort", sort)
	}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		v.Set("offset", strconv.Itoa(offset))
	}
	err := c.do(ctx, http.MethodGet, "/records", v, &out)
	return out, err
}

// GetRecord reads a single record by vault-relative path, including its body.
func (c *Client) GetRecord(ctx context.Context, path string) (Record, error) {
	var out Record
	err := c.do(ctx, http.MethodGet, "/record", url.Values{"path": {path}}, &out)
	return out, err
}

// CreateRecord creates a record in a collection. Fields are validated against
// the collection schema; body is the markdown body of the new note.
func (c *Client) CreateRecord(ctx context.Context, collection string, fields map[string]any, body string) (Record, error) {
	var out Record
	reqBody := map[string]any{"collection": collection, "fields": fields, "body": body}
	err := c.doJSON(ctx, http.MethodPost, "/records", nil, reqBody, &out)
	return out, err
}

// PatchRecord updates a record by path: fields are merged into its frontmatter (a
// nil value deletes a key) and body, when non-nil, replaces the markdown body.
func (c *Client) PatchRecord(ctx context.Context, path string, fields map[string]any, body *string) (Record, error) {
	var out Record
	reqBody := map[string]any{}
	if fields != nil {
		reqBody["fields"] = fields
	}
	if body != nil {
		reqBody["body"] = *body
	}
	err := c.doJSON(ctx, http.MethodPatch, "/record", url.Values{"path": {path}}, reqBody, &out)
	return out, err
}

// DeleteRecord removes a record by vault path and prunes it from the index.
func (c *Client) DeleteRecord(ctx context.Context, path string) error {
	var out map[string]any
	return c.do(ctx, http.MethodDelete, "/record", url.Values{"path": {path}}, &out)
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

// doJSON mirrors do but sends body as a JSON request payload with a
// Content-Type of application/json. A nil body sends no payload.
func (c *Client) doJSON(ctx context.Context, method, path string, q url.Values, body, out any) error {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s body: %w", path, err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(b)))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
	}
	return nil
}
