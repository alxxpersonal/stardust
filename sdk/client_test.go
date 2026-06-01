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
	require.NoError(t, os.WriteFile(filepath.Join(root, "note.md"),
		[]byte("---\ntitle: SDK Note\n---\n# SDK Note\ncontent about widgets and gadgets"), 0o644))

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
	require.Equal(t, 1, st.Notes)

	qr, err := c.Query(ctx, "widgets", 5)
	require.NoError(t, err)
	require.NotEmpty(t, qr.Hits)
	require.Equal(t, "note.md", qr.Hits[0].Path)

	n, err := c.GetNote(ctx, "note.md")
	require.NoError(t, err)
	require.Equal(t, "SDK Note", n.Title)
	require.Contains(t, n.Body, "widgets")

	g, err := c.Graph(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, g.Notes)

	b, err := c.Bundle(ctx, "widgets and gadgets", 1000)
	require.NoError(t, err)
	require.Contains(t, b.Markdown, "## Task")
}
