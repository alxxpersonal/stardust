package sdk_test

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
	"github.com/alxxpersonal/stardust/sdk"
)

func TestSDKAgainstAPI(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	mountsDir := config.Layout{Root: root}.Mounts()
	require.NoError(t, os.MkdirAll(filepath.Join(mountsDir, "gmail"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mountsDir, "gmail", "config.toml"),
		[]byte("command = \"gmail-mcp\"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "note.md"),
		[]byte("---\ntitle: SDK Note\n---\n# SDK Note\ncontent about widgets and gadgets, see [[related]]"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "related.md"),
		[]byte("---\ntitle: Related\n---\n# Related\nlinks back to [[note]]"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	srv := httptest.NewServer(api.New(svc))
	defer srv.Close()

	c := sdk.New(srv.URL)
	ctx := context.Background()

	st, err := c.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, st.Notes)

	qr, err := c.Query(ctx, "widgets", 5)
	require.NoError(t, err)
	require.NotEmpty(t, qr.Hits)
	require.Equal(t, "note.md", qr.Hits[0].Path)

	n, err := c.GetNote(ctx, "note.md")
	require.NoError(t, err)
	require.Equal(t, "SDK Note", n.Title)
	require.Contains(t, n.Body, "widgets")
	require.Len(t, n.LinkTargets, 1)
	require.Equal(t, "related", n.LinkTargets[0].Link)
	require.Equal(t, "related.md", n.LinkTargets[0].Path)

	g, err := c.Graph(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, g.Notes)
	require.NotEmpty(t, g.PageRank)
	require.NotEmpty(t, g.PageRank[0].Path)
	require.Greater(t, g.PageRank[0].Score, 0.0)

	ms, err := c.Mounts(ctx)
	require.NoError(t, err)
	require.Len(t, ms, 1)
	require.Equal(t, "gmail", ms[0].Name)
	require.Equal(t, "mcp", ms[0].Kind)
	require.Equal(t, "gmail-mcp", ms[0].Target)
	require.Equal(t, "query", ms[0].Tool) // defaulted

	b, err := c.Bundle(ctx, "widgets and gadgets", 1000)
	require.NoError(t, err)
	require.Contains(t, b.Markdown, "## Task")
}

func TestSDKCollectionsAndRecords(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))

	colDir := filepath.Join(root, ".stardust", "collections", "jobs")
	require.NoError(t, os.MkdirAll(colDir, 0o755))
	schema := "path = \"Jobs\"\n" +
		"description = \"job applications\"\n" +
		"[[fields]]\nname = \"company\"\ntype = \"string\"\nrequired = true\n" +
		"[[fields]]\nname = \"status\"\ntype = \"enum\"\nenum = [\"applied\", \"interview\", \"offer\"]\n" +
		"[[fields]]\nname = \"score\"\ntype = \"number\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(colDir, "config.toml"), []byte(schema), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	srv := httptest.NewServer(api.New(svc))
	defer srv.Close()

	c := sdk.New(srv.URL)
	ctx := context.Background()

	cols, err := c.ListCollections(ctx)
	require.NoError(t, err)
	require.Len(t, cols, 1)
	require.Equal(t, "jobs", cols[0].Name)
	require.Equal(t, "Jobs", cols[0].Path)
	require.Len(t, cols[0].Fields, 3)

	one, err := c.GetCollection(ctx, "jobs")
	require.NoError(t, err)
	require.Equal(t, "jobs", one.Name)

	acme, err := c.CreateRecord(ctx, "jobs",
		map[string]any{"company": "Acme", "status": "applied", "score": 7}, "first lead")
	require.NoError(t, err)
	require.Equal(t, "Jobs/acme.md", acme.Path)
	require.Contains(t, acme.Body, "first lead")

	_, err = c.CreateRecord(ctx, "jobs",
		map[string]any{"company": "Globex", "status": "interview", "score": 9}, "second lead")
	require.NoError(t, err)

	// Descending numeric sort.
	list, err := c.ListRecords(ctx, "jobs", nil, "-score", 0, 0)
	require.NoError(t, err)
	require.Equal(t, "Jobs", list.Folder)
	require.Len(t, list.Records, 2)
	require.Equal(t, float64(9), list.Records[0].Frontmatter["score"])

	// Numeric predicate compares numerically.
	list, err = c.ListRecords(ctx, "jobs", []sdk.Predicate{{Field: "score", Op: "gte", Value: "8"}}, "", 0, 0)
	require.NoError(t, err)
	require.Len(t, list.Records, 1)
	require.Equal(t, "Globex", list.Records[0].Frontmatter["company"])

	rec, err := c.GetRecord(ctx, "Jobs/acme.md")
	require.NoError(t, err)
	require.Equal(t, "applied", rec.Frontmatter["status"])
	require.Contains(t, rec.Body, "first lead")

	status := "offer"
	patched, err := c.PatchRecord(ctx, "Jobs/acme.md", map[string]any{"status": status}, nil)
	require.NoError(t, err)
	require.Equal(t, "offer", patched.Frontmatter["status"])

	list, err = c.ListRecords(ctx, "jobs", []sdk.Predicate{{Field: "status", Op: "eq", Value: "offer"}}, "", 0, 0)
	require.NoError(t, err)
	require.Len(t, list.Records, 1)
	require.Equal(t, "Acme", list.Records[0].Frontmatter["company"])
}
