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

// registerRegistryTools wires the MCP tools whose operations live in the shared
// registry, routing each through the router. Tool names, argument schemas, and
// result shapes are byte-identical to the direct-service registrations they
// supersede, so an agent's tool list and call results do not shift. The
// non-registry tools (query, bundle, graph, and the rest) stay on the direct
// service path until their operations join the registry.
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
}
