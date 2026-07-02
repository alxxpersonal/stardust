package mcp

import (
	"context"

	"github.com/creachadair/jrpc2/server"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alxxpersonal/stardust/internal/rpcserver"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/rpc"
)

// router dispatches MCP tool calls through the shared jrpc2 method registry
// instead of calling the service directly, so the MCP edge and the HTTP /rpc
// bridge resolve the same handlers (ADR 0003, ADR 0006). It is the thin adapter
// seam: MCP tool name -> slash method -> the one registry. jrpc2 owns decode,
// encode, id correlation, and the error band; the MCP go-sdk owns the
// initialize handshake, capability negotiation, and the isError-in-result split.
type router struct {
	local server.Local
}

// newRouter builds a router over an in-process jrpc2 client connected to the
// registry assembled from svc. The caller closes it via close.
func newRouter(svc *service.Service) *router {
	opts := &server.LocalOptions{Server: rpcserver.ServerOptions()}
	return &router{local: server.NewLocal(rpcserver.NewRegistry(svc), opts)}
}

// close tears down the in-memory jrpc2 client/server pair.
func (r *router) close() error { return r.local.Close() }

// call invokes a slash method on the registry with the typed params and decodes
// the typed result into out. A jrpc2 error (including the reserved
// -32000..-32099 infrastructure band) is returned unwrapped so the MCP layer can
// surface it as an isError result.
func (r *router) call(ctx context.Context, method string, params, out any) error {
	return r.local.Client.CallResult(ctx, method, params, out)
}

// registerRegistryTools wires every MCP tool through the router, so each tool
// call resolves over the shared jrpc2 registry (ADR 0003) rather than calling the
// service directly. The MCP layer is a thin adapter: tool name -> slash method ->
// the one registry. Tool names, argument schemas, and result shapes are
// byte-identical to the direct-service registrations they supersede, so an
// agent's tool list and call results do not shift. Each tool keeps its historical
// output type and decodes the registry's typed result into it; the registry's rpc
// result types carry the full service field set, so no field is dropped in the
// round-trip. The registration order is preserved so the emitted tool list is
// stable.
func registerRegistryTools(srv *sdkmcp.Server, r *router) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "status",
		Description: "Report the vault index status: note and chunk counts, last indexed commit, embedding model, and whether semantic search and reranking are active.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, service.Status, error) {
		var st service.Status
		err := r.call(ctx, "status", nil, &st)
		return nil, st, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_record",
		Description: "Read a single record by its vault-relative path. Returns the record's path, title, frontmatter columns, and full markdown body.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a getRecordArgs) (*sdkmcp.CallToolResult, service.Record, error) {
		var rec service.Record
		err := r.call(ctx, "record/get", rpc.RecordParams{Path: a.Path}, &rec)
		return nil, rec, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "create_record",
		Description: "Create a record in a collection. Fields are validated against the collection schema, written as the note's frontmatter, and the note is filed under the collection's folder with a unique slugged filename. The index updates automatically. Returns the created record.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a createRecordArgs) (*sdkmcp.CallToolResult, service.Record, error) {
		var rec service.Record
		err := r.call(ctx, "record/create", rpc.CreateRecordParams{
			Collection: a.Collection,
			Fields:     a.Fields,
			Body:       a.Body,
		}, &rec)
		return nil, rec, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "patch_record",
		Description: "Update a record by path: merge fields into its frontmatter (a null value deletes a key) and optionally replace its body. The merged frontmatter is validated against the owning collection's schema. The index updates automatically. Returns the updated record.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a patchRecordArgs) (*sdkmcp.CallToolResult, service.Record, error) {
		var rec service.Record
		err := r.call(ctx, "record/patch", rpc.PatchRecordParams{
			Path:   a.Path,
			Fields: a.Fields,
			Body:   a.Body,
		}, &rec)
		return nil, rec, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "query",
		Description: "Search the user's markdown vault with hybrid keyword + semantic retrieval. Use this whenever you need context from their notes before answering, or to check whether a note on a topic exists. Do NOT assume a note is absent without searching. Returns ranked notes with snippets.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a queryArgs) (*sdkmcp.CallToolResult, service.QueryResult, error) {
		limit := a.Limit
		if limit <= 0 {
			limit = 10
		}
		var res service.QueryResult
		err := r.call(ctx, "query", rpc.QueryParams{Query: a.Query, Limit: limit}, &res)
		return nil, res, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_note",
		Description: "Read a single note from the vault by its relative path (as returned by query). Returns the note's title, tags, links, and full markdown body.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a getNoteArgs) (*sdkmcp.CallToolResult, service.Note, error) {
		var n service.Note
		err := r.call(ctx, "note/get", rpc.NoteParams{Path: a.Path}, &n)
		return nil, n, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "bundle",
		Description: "Assemble a task-scoped context bundle: the notes most relevant to a task, expanded over the link graph with personalized PageRank and packed to a token budget. Use this to load yourself with the right context before starting work on a task.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a bundleArgs) (*sdkmcp.CallToolResult, service.BundleResult, error) {
		budget := a.Budget
		if budget <= 0 {
			budget = 4000
		}
		var res service.BundleResult
		err := r.call(ctx, "bundle", rpc.BundleParams{Task: a.Task, Budget: budget}, &res)
		return nil, res, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "graph",
		Description: "Report the vault's link graph: note and link counts, orphan notes (no links in or out), and broken wikilinks.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, service.GraphReport, error) {
		var rep service.GraphReport
		err := r.call(ctx, "graph", nil, &rep)
		return nil, rep, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "mounts",
		Description: "List the configured external-source mounts (context-mesh connectors). Returns each mount's name, kind, target command, args, and search tool. Use this to see which external sources a federated query can reach.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, mountsResult, error) {
		var ms []service.MountInfo
		err := r.call(ctx, "mount/list", nil, &ms)
		return nil, mountsResult{Mounts: ms}, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "check",
		Description: "Validate vault integrity: broken wikilinks and malformed frontmatter (errors), plus orphan notes, missing titles, and duplicate note names (warnings). Use this to verify the vault is healthy before relying on it or after edits.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, service.CheckResult, error) {
		var res service.CheckResult
		err := r.call(ctx, "check", nil, &res)
		return nil, res, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "digest",
		Description: "Summarize recent vault activity grouped by area, with open commitments (TODO, 'I'll do X'). Uses git as the change feed. Use this for a morning briefing or to catch up on what changed since last time.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a digestArgs) (*sdkmcp.CallToolResult, service.DigestResult, error) {
		var res service.DigestResult
		err := r.call(ctx, "digest", rpc.DigestParams{Since: a.Since, Advance: a.Advance}, &res)
		return nil, res, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "remember",
		Description: "Store a fact in the user's vault, add-only. Embeds it and appends to the most similar existing note, or creates a dated note under memory/. The index updates automatically. Use this to persist something you learned for future sessions.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a rememberArgs) (*sdkmcp.CallToolResult, service.RememberResult, error) {
		var res service.RememberResult
		err := r.call(ctx, "memory/remember", rpc.RememberParams{Fact: a.Fact}, &res)
		return nil, res, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "memory",
		Description: "Edit vault files with the memory verbs: view, create, str_replace, insert, delete, rename. Paths are confined to the vault and the index updates after each write. Prefer add-only edits; use 'remember' for quick fact capture.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a memoryArgs) (*sdkmcp.CallToolResult, memoryResult, error) {
		var out memoryResult
		err := r.call(ctx, "memory/edit", rpc.MemoryParams{
			Command: a.Command, Path: a.Path, Content: a.Content,
			OldStr: a.OldStr, NewStr: a.NewStr, Line: a.Line, Text: a.Text, Dest: a.Dest,
		}, &out)
		return nil, out, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_collections",
		Description: "List the structured collections defined over the vault. A collection is a vault folder paired with a typed schema; each note in it is a record and its frontmatter holds the typed columns. Returns each collection's name, folder, description, schema fields, and live record count.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, collectionsResult, error) {
		var cols []service.CollectionInfo
		err := r.call(ctx, "collection/list", nil, &cols)
		return nil, collectionsResult{Collections: cols}, err
	})

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_records",
		Description: "List records (notes) in a collection, filtered by frontmatter predicates and ordered by a sort field. Use this to query structured data like a table: filter on typed columns, sort, and paginate. Returns each record's path, title, and frontmatter columns.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a listRecordsArgs) (*sdkmcp.CallToolResult, service.RecordList, error) {
		preds, err := parsePredicates(a.Where)
		if err != nil {
			return nil, service.RecordList{}, err
		}
		var list service.RecordList
		err = r.call(ctx, "record/list", rpc.ListRecordsParams{
			Collection: a.Collection,
			Filter:     preds,
			Sort:       a.Sort,
			Limit:      a.Limit,
			Offset:     a.Offset,
		}, &list)
		return nil, list, err
	})
}
