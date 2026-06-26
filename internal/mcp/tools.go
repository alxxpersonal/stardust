package mcp

import (
	"context"
	"fmt"
	"strings"

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

type rememberArgs struct {
	Fact string `json:"fact" jsonschema:"the fact to store in the vault"`
}

type digestArgs struct {
	Since   string `json:"since,omitempty" jsonschema:"git SHA to diff from"`
	Advance bool   `json:"advance,omitempty" jsonschema:"advance the digest cursor to HEAD"`
}

type memoryArgs struct {
	Command string `json:"command" jsonschema:"one of: view, create, str_replace, insert, delete, rename"`
	Path    string `json:"path" jsonschema:"vault-relative file path"`
	Content string `json:"content,omitempty" jsonschema:"file content (create)"`
	OldStr  string `json:"old_str,omitempty" jsonschema:"text to replace (str_replace)"`
	NewStr  string `json:"new_str,omitempty" jsonschema:"replacement text (str_replace)"`
	Line    int    `json:"line,omitempty" jsonschema:"0-based line index (insert)"`
	Text    string `json:"text,omitempty" jsonschema:"text to insert (insert)"`
	Dest    string `json:"dest,omitempty" jsonschema:"destination path (rename)"`
}

type memoryResult struct {
	Result string `json:"result"`
}

// mountsResult wraps the mounts list so the MCP tool output schema is an object.
type mountsResult struct {
	Mounts []service.MountInfo `json:"mounts"`
}

// collectionsResult wraps the collections list so the MCP tool output schema is an object.
type collectionsResult struct {
	Collections []service.CollectionInfo `json:"collections"`
}

type listRecordsArgs struct {
	Collection string   `json:"collection" jsonschema:"the collection name to list records from"`
	Where      []string `json:"where,omitempty" jsonschema:"frontmatter filters as field:op:value, op is one of eq, ne, gt, gte, lt, lte, contains"`
	Sort       string   `json:"sort,omitempty" jsonschema:"sort field (a frontmatter key, or path / updated_at), prefix with - for descending"`
	Limit      int      `json:"limit,omitempty" jsonschema:"maximum number of records, 0 means no limit"`
	Offset     int      `json:"offset,omitempty" jsonschema:"number of records to skip for pagination"`
}

type getRecordArgs struct {
	Path string `json:"path" jsonschema:"vault-relative path to the record note, e.g. jobs/acme.md"`
}

type createRecordArgs struct {
	Collection string         `json:"collection" jsonschema:"the collection to create the record in"`
	Fields     map[string]any `json:"fields" jsonschema:"frontmatter columns for the record, validated against the collection schema"`
	Body       string         `json:"body,omitempty" jsonschema:"markdown body of the record note"`
}

type patchRecordArgs struct {
	Path   string         `json:"path" jsonschema:"vault-relative path to the record to patch"`
	Fields map[string]any `json:"fields,omitempty" jsonschema:"frontmatter columns to merge, a null value deletes that key"`
	Body   *string        `json:"body,omitempty" jsonschema:"new markdown body, omit to leave the body unchanged"`
}

// registerTools wires the Stardust tools over the core Service. The surface is
// small and the descriptions are specific so the client invokes them reliably.
// The record seam tools (status, get_record, create_record, patch_record) are
// registered through r so they resolve over the shared jrpc2 registry; the
// remaining tools call the service directly until their operations join the
// registry. Tool names and schemas are unchanged across the split.
func registerTools(server *sdkmcp.Server, svc *service.Service, r *router) {
	registerRegistryTools(server, r)

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
		Name:        "graph",
		Description: "Report the vault's link graph: note and link counts, orphan notes (no links in or out), and broken wikilinks.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, service.GraphReport, error) {
		rep, err := svc.Graph(ctx)
		return nil, rep, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "mounts",
		Description: "List the configured external-source mounts (context-mesh connectors). Returns each mount's name, kind, target command, args, and search tool. Use this to see which external sources a federated query can reach.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, mountsResult, error) {
		ms, err := svc.Mounts()
		return nil, mountsResult{Mounts: ms}, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "check",
		Description: "Validate vault integrity: broken wikilinks and malformed frontmatter (errors), plus orphan notes, missing titles, and duplicate note names (warnings). Use this to verify the vault is healthy before relying on it or after edits.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, service.CheckResult, error) {
		res, err := svc.Check(ctx)
		return nil, res, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "digest",
		Description: "Summarize recent vault activity grouped by area, with open commitments (TODO, 'I'll do X'). Uses git as the change feed. Use this for a morning briefing or to catch up on what changed since last time.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a digestArgs) (*sdkmcp.CallToolResult, service.DigestResult, error) {
		res, err := svc.Digest(ctx, a.Since, a.Advance)
		return nil, res, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "remember",
		Description: "Store a fact in the user's vault, add-only. Embeds it and appends to the most similar existing note, or creates a dated note under memory/. The index updates automatically. Use this to persist something you learned for future sessions.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a rememberArgs) (*sdkmcp.CallToolResult, service.RememberResult, error) {
		res, err := svc.Remember(ctx, a.Fact)
		return nil, res, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "memory",
		Description: "Edit vault files with the memory verbs: view, create, str_replace, insert, delete, rename. Paths are confined to the vault and the index updates after each write. Prefer add-only edits; use 'remember' for quick fact capture.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a memoryArgs) (*sdkmcp.CallToolResult, memoryResult, error) {
		out, err := svc.Memory(ctx, service.MemoryOp{
			Command: a.Command, Path: a.Path, Content: a.Content,
			Old: a.OldStr, New: a.NewStr, Line: a.Line, Text: a.Text, Dest: a.Dest,
		})
		return nil, memoryResult{Result: out}, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_collections",
		Description: "List the structured collections defined over the vault. A collection is a vault folder paired with a typed schema; each note in it is a record and its frontmatter holds the typed columns. Returns each collection's name, folder, description, schema fields, and live record count.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, collectionsResult, error) {
		cols, err := svc.ListCollections(ctx)
		return nil, collectionsResult{Collections: cols}, err
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_records",
		Description: "List records (notes) in a collection, filtered by frontmatter predicates and ordered by a sort field. Use this to query structured data like a table: filter on typed columns, sort, and paginate. Returns each record's path, title, and frontmatter columns.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a listRecordsArgs) (*sdkmcp.CallToolResult, service.RecordList, error) {
		preds, err := parsePredicates(a.Where)
		if err != nil {
			return nil, service.RecordList{}, err
		}
		list, err := svc.ListRecords(ctx, a.Collection, preds, a.Sort, a.Limit, a.Offset)
		return nil, list, err
	})

}

// parsePredicates turns "field:op:value" strings into service predicates. Only
// the first two colons separate the parts, so a value may itself contain colons.
// An empty field or op is rejected.
func parsePredicates(where []string) ([]service.Predicate, error) {
	if len(where) == 0 {
		return nil, nil
	}
	preds := make([]service.Predicate, 0, len(where))
	for _, w := range where {
		parts := strings.SplitN(w, ":", 3)
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid where clause %q: want field:op:value", w)
		}
		preds = append(preds, service.Predicate{Field: parts[0], Op: parts[1], Value: parts[2]})
	}
	return preds, nil
}
