package rpc_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/api"
	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/rpc"
)

// newClient stands an httptest server over a freshly indexed vault that holds a
// "jobs" collection, then returns a typed rpc.Client pointed at its /rpc
// endpoint. The fixture mirrors the rpcserver test vault so the client exercises
// the real service core over the jrpc2 HTTP bridge.
func newClient(t *testing.T) *rpc.Client {
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
`
	require.NoError(t, os.WriteFile(filepath.Join(schemaDir, "config.toml"), []byte(schema), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	srv := httptest.NewServer(api.New(svc))
	t.Cleanup(srv.Close)

	client := rpc.NewClient(srv.URL + "/rpc")
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestClientRecordGet(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)

	created, err := client.RecordCreate(ctx, rpc.CreateRecordParams{
		Collection: "jobs",
		Fields:     map[string]any{"company": "Acme", "status": "open"},
		Body:       "first body",
	})
	require.NoError(t, err)
	require.Equal(t, "jobs/acme.md", created.Path)

	got, err := client.RecordGet(ctx, rpc.RecordParams{Path: created.Path})
	require.NoError(t, err)
	require.Equal(t, "jobs/acme.md", got.Path)
	require.Equal(t, "open", got.Frontmatter["status"])
	require.Contains(t, got.Body, "first body")
}

func TestClientStatusAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)

	st, err := client.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, st.Notes)
	require.Equal(t, rpc.ContractVersion, "1")

	created, err := client.RecordCreate(ctx, rpc.CreateRecordParams{
		Collection: "jobs",
		Fields:     map[string]any{"company": "Globex", "status": "open"},
		Body:       "body",
	})
	require.NoError(t, err)

	list, err := client.RecordList(ctx, rpc.ListRecordsParams{Collection: "jobs"})
	require.NoError(t, err)
	require.Len(t, list.Records, 1)
	require.Equal(t, "jobs", list.Collection)

	body := "patched body"
	patched, err := client.RecordPatch(ctx, rpc.PatchRecordParams{
		Path:   created.Path,
		Fields: map[string]any{"status": "closed"},
		Body:   &body,
	})
	require.NoError(t, err)
	require.Equal(t, "closed", patched.Frontmatter["status"])
	require.Contains(t, patched.Body, "patched body")

	del, err := client.RecordDelete(ctx, rpc.RecordParams{Path: created.Path})
	require.NoError(t, err)
	require.Equal(t, created.Path, del.Path)
	require.Equal(t, "deleted", del.Status)
}
