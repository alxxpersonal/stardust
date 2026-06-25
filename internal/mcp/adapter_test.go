package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

// jobsVault builds a temp vault with a "jobs" collection schema and returns its
// root, mirroring the rpcserver fixture so the MCP adapter runs its
// registry-backed tools against the real service core.
func jobsVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	schemaDir := filepath.Join(root, ".stardust", "collections", "jobs")
	require.NoError(t, os.MkdirAll(schemaDir, 0o755))
	schema := `
path = "jobs"
description = "job applications"

[[fields]]
name = "company"
type = "string"
required = true

[[fields]]
name = "status"
type = "enum"
enum = ["open", "closed"]

[[fields]]
name = "score"
type = "number"
`
	require.NoError(t, os.WriteFile(filepath.Join(schemaDir, "config.toml"), []byte(schema), 0o644))
	return root
}

// connectMCP stands an in-memory MCP server over a service opened on root, wired
// with the registry-routed tool set, and returns an initialized client session.
// The caller's cleanup closes both ends and the service.
func connectMCP(t *testing.T, root string) *sdkmcp.ClientSession {
	t.Helper()
	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "stardust", Version: version}, nil)
	r := newRouter(svc)
	t.Cleanup(func() { _ = r.close() })
	registerRegistryTools(server, r)

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestAdapterCreateRecordThroughRegistry asserts the create_record MCP tool
// resolves through the shared jrpc2 registry (record/create) and preserves both
// its MCP tool name and its result shape.
func TestAdapterCreateRecordThroughRegistry(t *testing.T) {
	ctx := context.Background()
	cs := connectMCP(t, jobsVault(t))

	res, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "create_record",
		Arguments: map[string]any{
			"collection": "jobs",
			"fields":     map[string]any{"company": "Acme", "status": "open", "score": float64(8)},
			"body":       "first body",
		},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var rec service.Record
	require.NoError(t, json.Unmarshal(mustMarshal(t, res.StructuredContent), &rec))
	require.Equal(t, "jobs/acme.md", rec.Path)
	require.Equal(t, "open", rec.Frontmatter["status"])
	require.Contains(t, rec.Body, "first body")
}

// TestAdapterPreservesToolNames asserts the registry-routed surface keeps the
// existing MCP tool names; an agent's tool list must not shift under the adapter.
func TestAdapterPreservesToolNames(t *testing.T) {
	ctx := context.Background()
	cs := connectMCP(t, jobsVault(t))

	list, err := cs.ListTools(ctx, nil)
	require.NoError(t, err)
	names := make(map[string]bool, len(list.Tools))
	for _, tool := range list.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"status", "create_record", "get_record", "patch_record"} {
		require.True(t, names[want], "missing MCP tool %q", want)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
