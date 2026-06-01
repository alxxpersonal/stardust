package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alxxpersonal/stardust/internal/service"
)

type queryArgs struct {
	Query string `json:"query" jsonschema:"the search query"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results, default 10"`
}

type getNoteArgs struct {
	Path string `json:"path" jsonschema:"vault-relative path to the note, e.g. notes/foo.md"`
}

type bundleArgs struct {
	Task   string `json:"task" jsonschema:"the task description to assemble context for"`
	Budget int    `json:"budget,omitempty" jsonschema:"approximate token budget, default 4000"`
}

// registerTools wires the Stardust tools over the core Service. The surface is
// small and the descriptions are specific so the client invokes them reliably.
func registerTools(server *sdkmcp.Server, svc *service.Service) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "query",
		Description: "Search the user's markdown vault with hybrid keyword + semantic retrieval. Use this whenever you need context from their notes before answering, or to check whether a note on a topic exists. Do NOT assume a note is absent without searching. Returns ranked notes with snippets.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a queryArgs) (*sdkmcp.CallToolResult, service.QueryResult, error) {
		limit := a.Limit
		if limit <= 0 {
			limit = 10
		}
		res, err := svc.Query(ctx, a.Query, limit)
		return nil, res, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_note",
		Description: "Read a single note from the vault by its relative path (as returned by query). Returns the note's title, tags, links, and full markdown body.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a getNoteArgs) (*sdkmcp.CallToolResult, service.Note, error) {
		n, err := svc.GetNote(ctx, a.Path)
		return nil, n, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "bundle",
		Description: "Assemble a task-scoped context bundle: the notes most relevant to a task, expanded over the link graph with personalized PageRank and packed to a token budget. Use this to load yourself with the right context before starting work on a task.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a bundleArgs) (*sdkmcp.CallToolResult, service.BundleResult, error) {
		budget := a.Budget
		if budget <= 0 {
			budget = 4000
		}
		res, err := svc.Bundle(ctx, a.Task, budget)
		return nil, res, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "status",
		Description: "Report the vault index status: note and chunk counts, last indexed commit, embedding model, and whether semantic search and reranking are active.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, service.Status, error) {
		st, err := svc.Status(ctx)
		return nil, st, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "graph",
		Description: "Report the vault's link graph: note and link counts, orphan notes (no links in or out), and broken wikilinks.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, service.GraphReport, error) {
		rep, err := svc.Graph(ctx)
		return nil, rep, err
	})
}
