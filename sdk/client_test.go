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
