package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

// callTool invokes an MCP tool by name with args and asserts a non-error result,
// so each round-trip test proves the tool resolves through the registry-backed
// adapter and returns a usable structured payload.
func callTool(t *testing.T, cs *sdkmcp.ClientSession, name string, args map[string]any) *sdkmcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	require.False(t, res.IsError, "tool %q returned an error result", name)
	return res
}

// decodeStructured decodes a tool call's structured content into out, mirroring
// how an MCP client reads the typed result. It round-trips through JSON so the
// assertion runs over the exact wire bytes the adapter emits.
func decodeStructured(t *testing.T, res *sdkmcp.CallToolResult, out any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(mustMarshal(t, res.StructuredContent), out))
}

// writeVaultFile writes a file at a vault-relative path, creating parent dirs, so
// tests can seed notes on disk before opening the service over the vault.
func writeVaultFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// gitInitVault initializes a git repo at root and commits its current tree, so
// the digest tool (which reads git as its change feed) has a HEAD to diff.
func gitInitVault(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}
}

// seedRecord creates a record through the create_record tool and returns the
// decoded result, so tests that read records back share one seeding path.
func seedRecord(t *testing.T, cs *sdkmcp.ClientSession) service.Record {
	t.Helper()
	res := callTool(t, cs, "create_record", map[string]any{
		"collection": "jobs",
		"fields":     map[string]any{"company": "Acme", "status": "open", "score": float64(8)},
		"body":       "first body",
	})
	var rec service.Record
	decodeStructured(t, res, &rec)
	return rec
}

// TestAdapterStatusThroughRegistry asserts the status tool resolves over the
// registry and its Status result decodes with the vault root echoed back.
func TestAdapterStatusThroughRegistry(t *testing.T) {
	root := jobsVault(t)
	cs := connectMCP(t, root)

	res := callTool(t, cs, "status", nil)
	var st service.Status
	decodeStructured(t, res, &st)
	require.Equal(t, root, st.Root)
}

// TestAdapterGetRecordThroughRegistry asserts get_record routes through
// record/get and reads back a record created in the same session.
func TestAdapterGetRecordThroughRegistry(t *testing.T) {
	cs := connectMCP(t, jobsVault(t))
	seedRecord(t, cs)

	res := callTool(t, cs, "get_record", map[string]any{"path": "jobs/acme.md"})
	var rec service.Record
	decodeStructured(t, res, &rec)
	require.Equal(t, "jobs/acme.md", rec.Path)
	require.Equal(t, "open", rec.Frontmatter["status"])
	require.Contains(t, rec.Body, "first body")
}

// TestAdapterPatchRecordThroughRegistry asserts patch_record routes through
// record/patch, merging a frontmatter change into an existing record.
func TestAdapterPatchRecordThroughRegistry(t *testing.T) {
	cs := connectMCP(t, jobsVault(t))
	seedRecord(t, cs)

	res := callTool(t, cs, "patch_record", map[string]any{
		"path":   "jobs/acme.md",
		"fields": map[string]any{"status": "closed"},
	})
	var rec service.Record
	decodeStructured(t, res, &rec)
	require.Equal(t, "jobs/acme.md", rec.Path)
	require.Equal(t, "closed", rec.Frontmatter["status"])
}

// TestAdapterQueryThroughRegistry asserts query routes through the registry and
// its enriched QueryResult (retrieval_mode, reranked, hits) decodes. The
// retrieval mode is one of the two enum values depending on embedder
// availability, which the field round-trips either way.
func TestAdapterQueryThroughRegistry(t *testing.T) {
	cs := connectMCP(t, jobsVault(t))
	seedRecord(t, cs)

	res := callTool(t, cs, "query", map[string]any{"query": "first body", "limit": float64(5)})
	var qr service.QueryResult
	decodeStructured(t, res, &qr)
	require.Equal(t, "first body", qr.Query)
	require.Contains(t, []string{service.RetrievalHybridSemantic, service.RetrievalFTSOnly}, qr.RetrievalMode)
	require.NotNil(t, qr.Hits)
}

// TestAdapterGetNoteThroughRegistry asserts get_note routes through note/get and
// its Note result decodes with resolved link targets.
func TestAdapterGetNoteThroughRegistry(t *testing.T) {
	root := jobsVault(t)
	writeVaultFile(t, root, "notes/source.md", "---\ntitle: Source\ntags: [alpha]\n---\n# Source\nsee [[target]] and [[ghost]]")
	writeVaultFile(t, root, "notes/target.md", "---\ntitle: Target\n---\n# Target\nbody")
	cs := connectMCP(t, root)

	res := callTool(t, cs, "get_note", map[string]any{"path": "notes/source.md"})
	var n service.Note
	decodeStructured(t, res, &n)
	require.Equal(t, "notes/source.md", n.Path)
	require.Equal(t, "Source", n.Title)
	require.Equal(t, []string{"target", "ghost"}, n.Links)
	require.Len(t, n.LinkTargets, 2)
	require.Equal(t, "notes/target.md", n.LinkTargets[0].Path)
	require.Equal(t, "", n.LinkTargets[1].Path)
}

// TestAdapterBundleThroughRegistry asserts bundle routes through the registry and
// its enriched BundleResult (retrieval_mode, commits_behind, item provenance)
// decodes.
func TestAdapterBundleThroughRegistry(t *testing.T) {
	cs := connectMCP(t, jobsVault(t))
	seedRecord(t, cs)

	res := callTool(t, cs, "bundle", map[string]any{"task": "first body", "budget": float64(1000)})
	var br service.BundleResult
	decodeStructured(t, res, &br)
	require.Equal(t, "first body", br.Task)
	require.Contains(t, []string{service.RetrievalHybridSemantic, service.RetrievalFTSOnly}, br.RetrievalMode)
	require.NotNil(t, br.Items)
}

// TestAdapterGraphThroughRegistry asserts graph routes through the registry and
// its GraphReport decodes, including the enriched BrokenLink.Kind field.
func TestAdapterGraphThroughRegistry(t *testing.T) {
	root := jobsVault(t)
	writeVaultFile(t, root, "notes/source.md", "---\ntitle: Source\n---\n# Source\nsee [[target]] and [[ghost]]")
	writeVaultFile(t, root, "notes/target.md", "---\ntitle: Target\n---\n# Target\nbody")
	cs := connectMCP(t, root)

	res := callTool(t, cs, "graph", nil)
	var rep service.GraphReport
	decodeStructured(t, res, &rep)
	require.Equal(t, 2, rep.Notes)
	require.NotEmpty(t, rep.Broken)
	require.Equal(t, "missing", firstBrokenTarget(rep, "ghost"))
	for _, b := range rep.Broken {
		require.NotEmpty(t, b.Kind, "broken link kind must round-trip")
	}
}

// firstBrokenTarget returns "missing" when a broken link for the given unresolved
// target is present, else the empty string, so the graph assertion reads cleanly.
func firstBrokenTarget(rep service.GraphReport, target string) string {
	for _, b := range rep.Broken {
		if b.Target == target {
			return "missing"
		}
	}
	return ""
}

// TestAdapterMountsThroughRegistry asserts mounts routes through mount/list and
// the adapter wraps the list in its {mounts: [...]} object shape.
func TestAdapterMountsThroughRegistry(t *testing.T) {
	root := jobsVault(t)
	mountsDir := config.Layout{Root: root}.Mounts()
	require.NoError(t, os.MkdirAll(filepath.Join(mountsDir, "gmail"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mountsDir, "gmail", "config.toml"),
		[]byte("command = \"gmail-mcp\"\nargs = [\"serve\"]\ntool = \"search\"\n"), 0o644))
	cs := connectMCP(t, root)

	res := callTool(t, cs, "mounts", nil)
	var out mountsResult
	decodeStructured(t, res, &out)
	require.Len(t, out.Mounts, 1)
	require.Equal(t, "gmail", out.Mounts[0].Name)
	require.Equal(t, "mcp", out.Mounts[0].Kind)
	require.Equal(t, "gmail-mcp", out.Mounts[0].Target)
	require.Equal(t, "search", out.Mounts[0].Tool)
}

// TestAdapterCheckThroughRegistry asserts check routes through the registry and
// its CheckResult decodes, surfacing a broken wikilink as an issue.
func TestAdapterCheckThroughRegistry(t *testing.T) {
	root := jobsVault(t)
	writeVaultFile(t, root, "notes/broken.md", "---\ntitle: Broken\n---\n# Broken\nsee [[ghost]]")
	cs := connectMCP(t, root)

	res := callTool(t, cs, "check", nil)
	var cr service.CheckResult
	decodeStructured(t, res, &cr)
	require.NotEmpty(t, cr.Issues)
	require.Contains(t, cr.Markdown, "Vault check")
}

// TestAdapterDigestThroughRegistry asserts digest routes through the registry.
// Digest reads git as its change feed, so the vault is committed first.
func TestAdapterDigestThroughRegistry(t *testing.T) {
	root := jobsVault(t)
	writeVaultFile(t, root, "todo.md", "---\ntitle: Todo\n---\n# Todo\nTODO: ship the digest feature.")
	gitInitVault(t, root)
	cs := connectMCP(t, root)

	res := callTool(t, cs, "digest", map[string]any{})
	var dr service.DigestResult
	decodeStructured(t, res, &dr)
	require.GreaterOrEqual(t, dr.Changed, 1)
	require.Contains(t, dr.Markdown, "Digest")
}

// TestAdapterRememberThroughRegistry asserts remember routes through
// memory/remember. Without a live embedder it creates a dated note under memory/.
func TestAdapterRememberThroughRegistry(t *testing.T) {
	cs := connectMCP(t, jobsVault(t))

	res := callTool(t, cs, "remember", map[string]any{"fact": "the deploy key rotates every 90 days"})
	var rr service.RememberResult
	decodeStructured(t, res, &rr)
	require.Equal(t, "created", rr.Action)
	require.True(t, strings.HasPrefix(rr.Path, "memory/"), "remember path %q must land under memory/", rr.Path)
}

// TestAdapterMemoryThroughRegistry asserts memory routes through memory/edit,
// exercising a create then a view verb and the adapter's {result: ...} wrapper.
func TestAdapterMemoryThroughRegistry(t *testing.T) {
	cs := connectMCP(t, jobsVault(t))

	created := callTool(t, cs, "memory", map[string]any{
		"command": "create",
		"path":    "notes/x.md",
		"content": "---\ntitle: X\n---\n# X\nalpha content",
	})
	var createOut memoryResult
	decodeStructured(t, created, &createOut)
	require.Contains(t, createOut.Result, "created")

	viewed := callTool(t, cs, "memory", map[string]any{"command": "view", "path": "notes/x.md"})
	var viewOut memoryResult
	decodeStructured(t, viewed, &viewOut)
	require.Contains(t, viewOut.Result, "alpha content")
}

// TestAdapterListCollectionsThroughRegistry asserts list_collections routes
// through collection/list and the adapter wraps it in {collections: [...]}.
func TestAdapterListCollectionsThroughRegistry(t *testing.T) {
	cs := connectMCP(t, jobsVault(t))

	res := callTool(t, cs, "list_collections", nil)
	var out collectionsResult
	decodeStructured(t, res, &out)
	require.NotEmpty(t, out.Collections)
	found := false
	for _, c := range out.Collections {
		if c.Name == "jobs" {
			found = true
			require.Equal(t, "jobs", c.Path)
		}
	}
	require.True(t, found, "jobs collection must be listed")
}

// TestAdapterListRecordsThroughRegistry asserts list_records routes through
// record/list, reading back a record created in the same session.
func TestAdapterListRecordsThroughRegistry(t *testing.T) {
	cs := connectMCP(t, jobsVault(t))
	seedRecord(t, cs)

	res := callTool(t, cs, "list_records", map[string]any{"collection": "jobs"})
	var list service.RecordList
	decodeStructured(t, res, &list)
	require.Equal(t, "jobs", list.Collection)
	require.Len(t, list.Records, 1)
	require.Equal(t, "jobs/acme.md", list.Records[0].Path)
}
