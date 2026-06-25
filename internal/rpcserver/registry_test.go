package rpcserver_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/creachadair/jrpc2/server"
	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/rpcserver"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/rpc"
)

// jobsVault builds a temp vault with a "jobs" collection schema and returns its
// root, mirroring the service-package test fixture so the registry runs against
// the real service core.
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

// localFromRegistry opens a service over root, builds the registry, and stands
// an in-memory jrpc2 client/server pair over it. The caller closes the returned
// Local; the service is closed via t.Cleanup.
func localFromRegistry(t *testing.T, root string) server.Local {
	t.Helper()
	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return server.NewLocal(rpcserver.NewRegistry(svc), nil)
}

func TestRegistryRecordCreate(t *testing.T) {
	ctx := context.Background()
	loc := localFromRegistry(t, jobsVault(t))
	defer func() { _ = loc.Close() }()

	var rec rpc.Record
	err := loc.Client.CallResult(ctx, "record/create", rpc.CreateRecordParams{
		Collection: "jobs",
		Fields:     map[string]any{"company": "Acme", "status": "open", "score": float64(8)},
		Body:       "first body",
	}, &rec)
	require.NoError(t, err)

	require.Equal(t, "jobs/acme.md", rec.Path)
	require.Equal(t, "open", rec.Frontmatter["status"])
	require.Contains(t, rec.Body, "first body")
}

// TestRegistryMethodSet pins the registry's method set. NewRegistry MUST expose
// exactly the canonical slash names for the record seam, no more and no fewer.
// A removed, renamed, or silently added method fails this test.
func TestRegistryMethodSet(t *testing.T) {
	svc, err := service.Open(context.Background(), jobsVault(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	want := []string{
		"record/create",
		"record/delete",
		"record/get",
		"record/list",
		"record/patch",
		"status",
	}

	reg := rpcserver.NewRegistry(svc)
	got := make([]string, 0, len(reg))
	for name := range reg {
		got = append(got, name)
	}
	sort.Strings(got)

	require.Equal(t, want, got)
}
