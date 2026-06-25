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
