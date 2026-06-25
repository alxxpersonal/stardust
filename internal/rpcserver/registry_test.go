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
	opts := &server.LocalOptions{Server: rpcserver.ServerOptions()}
	return server.NewLocal(rpcserver.NewRegistry(svc), opts)
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
// exactly the canonical slash names for the full operation set plus the reserved
// rpc.discover method, no more and no fewer. A removed, renamed, or silently added
// method fails this test. The full set totals twenty-one methods.
func TestRegistryMethodSet(t *testing.T) {
	svc, err := service.Open(context.Background(), jobsVault(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	want := []string{
		"archive",
		"bundle",
		"check",
		"collection/get",
		"collection/list",
		"cron/list",
		"cron/run",
		"digest",
		"graph",
		"index/rebuild",
		"index/run",
		"mount/list",
		"note/get",
		"query",
		"record/create",
		"record/delete",
		"record/get",
		"record/list",
		"record/patch",
		"rpc.discover",
		"status",
	}

	reg := rpcserver.NewRegistry(svc)
	got := make([]string, 0, len(reg))
	for name := range reg {
		got = append(got, name)
	}
	sort.Strings(got)

	require.Equal(t, want, got)
	require.Len(t, got, 21)
}

// TestRegistryDiscover pins rpc.discover. Calling it returns an OpenRPC document
// whose method list names every method the registry exposes, including
// rpc.discover itself, with a non-empty one-line summary per method. A method
// added to or removed from the registry without updating the document's source
// fails this test.
func TestRegistryDiscover(t *testing.T) {
	ctx := context.Background()
	svc, err := service.Open(ctx, jobsVault(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	reg := rpcserver.NewRegistry(svc)
	loc := localFromRegistry(t, jobsVault(t))
	defer func() { _ = loc.Close() }()

	var doc rpc.OpenRPCDoc
	require.NoError(t, loc.Client.CallResult(ctx, "rpc.discover", nil, &doc))

	require.Equal(t, rpc.OpenRPCVersion, doc.OpenRPC)
	require.Equal(t, rpc.ContractVersion, doc.Info.Version)

	docNames := make([]string, 0, len(doc.Methods))
	for _, m := range doc.Methods {
		require.NotEmpty(t, m.Summary, "method %q must carry a summary", m.Name)
		docNames = append(docNames, m.Name)
	}
	sort.Strings(docNames)

	regNames := reg.Names()
	sort.Strings(regNames)

	require.Equal(t, regNames, docNames, "discover document must list exactly the registry methods")
	require.Len(t, doc.Methods, 21)
	require.Contains(t, docNames, "rpc.discover")
}
