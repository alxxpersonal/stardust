package rpc

import (
	"context"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"
)

// Client is a typed JSON-RPC 2.0 client for the Stardust record seam. It wraps a
// jrpc2 client over an HTTP channel (jhttp), so callers invoke the canonical
// slash method names through compiler-checked Params and Result types instead of
// map[string]any. jrpc2 owns request framing, id correlation, and error decoding
// (ADR 0006).
type Client struct {
	rpc *jrpc2.Client
}

// NewClient returns a Client that posts JSON-RPC requests to url, the server's
// /rpc endpoint (e.g. http://127.0.0.1:7777/rpc). The caller must Close it to
// release the underlying channel.
func NewClient(url string) *Client {
	ch := jhttp.NewChannel(url, nil)
	return &Client{rpc: jrpc2.NewClient(ch, nil)}
}

// Close shuts down the underlying jrpc2 client and its HTTP channel.
func (c *Client) Close() error { return c.rpc.Close() }

// Status reports index health. The method takes no params.
func (c *Client) Status(ctx context.Context) (StatusResult, error) {
	var out StatusResult
	err := c.rpc.CallResult(ctx, "status", nil, &out)
	return out, err
}

// RecordCreate validates fields against the collection schema and writes a new
// note, returning the created record.
func (c *Client) RecordCreate(ctx context.Context, p CreateRecordParams) (Record, error) {
	var out Record
	err := c.rpc.CallResult(ctx, "record/create", p, &out)
	return out, err
}

// RecordGet reads a single record by its vault-relative path, including its body.
func (c *Client) RecordGet(ctx context.Context, p RecordParams) (Record, error) {
	var out Record
	err := c.rpc.CallResult(ctx, "record/get", p, &out)
	return out, err
}

// RecordList lists records in a collection, filtered and ordered by the supplied
// params.
func (c *Client) RecordList(ctx context.Context, p ListRecordsParams) (RecordList, error) {
	var out RecordList
	err := c.rpc.CallResult(ctx, "record/list", p, &out)
	return out, err
}

// RecordPatch merges fields into a record's frontmatter, optionally replaces its
// body, and returns the updated record.
func (c *Client) RecordPatch(ctx context.Context, p PatchRecordParams) (Record, error) {
	var out Record
	err := c.rpc.CallResult(ctx, "record/patch", p, &out)
	return out, err
}

// RecordDelete archives a record by path and reports the terminal status.
func (c *Client) RecordDelete(ctx context.Context, p RecordParams) (DeleteResult, error) {
	var out DeleteResult
	err := c.rpc.CallResult(ctx, "record/delete", p, &out)
	return out, err
}

// Query runs hybrid retrieval and returns the ranked hits.
func (c *Client) Query(ctx context.Context, p QueryParams) (QueryResult, error) {
	var out QueryResult
	err := c.rpc.CallResult(ctx, "query", p, &out)
	return out, err
}

// Bundle assembles a token-budgeted context bundle for a task.
func (c *Client) Bundle(ctx context.Context, p BundleParams) (BundleResult, error) {
	var out BundleResult
	err := c.rpc.CallResult(ctx, "bundle", p, &out)
	return out, err
}

// Graph derives the link graph and returns its summary. The method takes no
// params.
func (c *Client) Graph(ctx context.Context) (GraphResult, error) {
	var out GraphResult
	err := c.rpc.CallResult(ctx, "graph", nil, &out)
	return out, err
}

// Digest summarizes vault activity since a commit cursor, optionally advancing
// the stored cursor to HEAD.
func (c *Client) Digest(ctx context.Context, p DigestParams) (DigestResult, error) {
	var out DigestResult
	err := c.rpc.CallResult(ctx, "digest", p, &out)
	return out, err
}

// Check runs the vault integrity check. The method takes no params.
func (c *Client) Check(ctx context.Context) (CheckResult, error) {
	var out CheckResult
	err := c.rpc.CallResult(ctx, "check", nil, &out)
	return out, err
}

// NoteGet reads the parsed note at a vault-relative path.
func (c *Client) NoteGet(ctx context.Context, p NoteParams) (Note, error) {
	var out Note
	err := c.rpc.CallResult(ctx, "note/get", p, &out)
	return out, err
}

// CollectionList lists every configured collection with a live record count. The
// method takes no params.
func (c *Client) CollectionList(ctx context.Context) ([]Collection, error) {
	var out []Collection
	err := c.rpc.CallResult(ctx, "collection/list", nil, &out)
	return out, err
}

// CollectionGet reads a single collection by name.
func (c *Client) CollectionGet(ctx context.Context, p CollectionParams) (Collection, error) {
	var out Collection
	err := c.rpc.CallResult(ctx, "collection/get", p, &out)
	return out, err
}

// MountList returns the configured mounts. The method takes no params.
func (c *Client) MountList(ctx context.Context) ([]Mount, error) {
	var out []Mount
	err := c.rpc.CallResult(ctx, "mount/list", nil, &out)
	return out, err
}

// IndexRun incrementally indexes the vault; a non-empty Since uses the git-diff
// fast path.
func (c *Client) IndexRun(ctx context.Context, p IndexParams) (IndexStats, error) {
	var out IndexStats
	err := c.rpc.CallResult(ctx, "index/run", p, &out)
	return out, err
}

// IndexRebuild clears the derived cache and reindexes from scratch. The method
// takes no params.
func (c *Client) IndexRebuild(ctx context.Context) (IndexStats, error) {
	var out IndexStats
	err := c.rpc.CallResult(ctx, "index/rebuild", nil, &out)
	return out, err
}

// Archive snapshots the vault's git history into Dest (empty uses the default
// archives directory).
func (c *Client) Archive(ctx context.Context, p ArchiveParams) (ArchiveResult, error) {
	var out ArchiveResult
	err := c.rpc.CallResult(ctx, "archive", p, &out)
	return out, err
}

// CronList returns the configured cron jobs flattened for the wire. The method
// takes no params.
func (c *Client) CronList(ctx context.Context) ([]CronJob, error) {
	var out []CronJob
	err := c.rpc.CallResult(ctx, "cron/list", nil, &out)
	return out, err
}

// CronRun executes a cron job by name and returns the buffered run output. The
// server streams the run to an io.Writer and buffers it into the result string.
func (c *Client) CronRun(ctx context.Context, p CronRunParams) (CronRunResult, error) {
	var out CronRunResult
	err := c.rpc.CallResult(ctx, "cron/run", p, &out)
	return out, err
}

// Remember stores a fact add-only and reports where it landed (appended to the
// nearest note or created under memory/).
func (c *Client) Remember(ctx context.Context, p RememberParams) (RememberResult, error) {
	var out RememberResult
	err := c.rpc.CallResult(ctx, "memory/remember", p, &out)
	return out, err
}

// MemoryEdit applies one memory verb to a vault file and returns the outcome
// line.
func (c *Client) MemoryEdit(ctx context.Context, p MemoryParams) (MemoryResult, error) {
	var out MemoryResult
	err := c.rpc.CallResult(ctx, "memory/edit", p, &out)
	return out, err
}
