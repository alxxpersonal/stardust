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

// TestClientFullOperationSet exercises the typed client methods added for the
// full operation set, proving each new method is wired to its slash route and
// decodes a typed result over the jrpc2 HTTP bridge. The temp fixture is not a
// git repo, so digest and archive surface their "not a git repository" error
// through the typed method rather than panicking; the call path is what is under
// test.
func TestClientFullOperationSet(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)

	_, err := client.RecordCreate(ctx, rpc.CreateRecordParams{
		Collection: "jobs",
		Fields:     map[string]any{"company": "Initech", "status": "open"},
		Body:       "initech body",
	})
	require.NoError(t, err)

	cols, err := client.CollectionList(ctx)
	require.NoError(t, err)
	require.Len(t, cols, 1)
	require.Equal(t, "jobs", cols[0].Name)

	col, err := client.CollectionGet(ctx, rpc.CollectionParams{Name: "jobs"})
	require.NoError(t, err)
	require.Equal(t, "jobs", col.Name)
	require.Equal(t, 1, col.Records)

	note, err := client.NoteGet(ctx, rpc.NoteParams{Path: "jobs/initech.md"})
	require.NoError(t, err)
	require.Equal(t, "jobs/initech.md", note.Path)

	qr, err := client.Query(ctx, rpc.QueryParams{Query: "initech", Limit: 5})
	require.NoError(t, err)
	require.Equal(t, "initech", qr.Query)

	br, err := client.Bundle(ctx, rpc.BundleParams{Task: "initech", Budget: 1000})
	require.NoError(t, err)
	require.Equal(t, "initech", br.Task)

	graph, err := client.Graph(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, graph.Notes)

	check, err := client.Check(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, check.Errors, 0)

	mounts, err := client.MountList(ctx)
	require.NoError(t, err)
	require.Empty(t, mounts)

	jobs, err := client.CronList(ctx)
	require.NoError(t, err)
	require.Empty(t, jobs)

	stats, err := client.IndexRun(ctx, rpc.IndexParams{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.Indexed, 0)

	rebuilt, err := client.IndexRebuild(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, rebuilt.Indexed)

	_, err = client.Digest(ctx, rpc.DigestParams{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a git repository")

	_, err = client.Archive(ctx, rpc.ArchiveParams{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a git repository")

	_, err = client.CronRun(ctx, rpc.CronRunParams{Name: "missing"})
	require.Error(t, err)
}

// TestClientMemoryOps exercises the memory/edit and memory/remember client
// methods over the HTTP bridge, proving each is wired to its slash route and
// decodes a typed result. The temp vault has no embedding backend, so Remember
// falls through to creating a dated note under memory/ rather than appending.
func TestClientMemoryOps(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)

	edited, err := client.MemoryEdit(ctx, rpc.MemoryParams{
		Command: "create",
		Path:    "memory/scratch.md",
		Content: "---\ntitle: Scratch\n---\n\nfirst line\n",
	})
	require.NoError(t, err)
	require.Equal(t, "created memory/scratch.md", edited.Result)

	viewed, err := client.MemoryEdit(ctx, rpc.MemoryParams{Command: "view", Path: "memory/scratch.md"})
	require.NoError(t, err)
	require.Contains(t, viewed.Result, "first line")

	remembered, err := client.Remember(ctx, rpc.RememberParams{Fact: "the registry owns every transport"})
	require.NoError(t, err)
	require.Equal(t, "created", remembered.Action)
	require.Contains(t, remembered.Path, "memory/")
}
