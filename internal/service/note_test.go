package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

func TestGetNoteResolvesLinkTargets(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "notes"), 0o755))
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	// source links to a note in a subfolder (resolved by base name) and a missing one.
	write("source.md", "---\ntitle: Source\n---\n# Source\nsee [[target]] and [[ghost]]")
	write("notes/target.md", "---\ntitle: Target\n---\n# Target\nbody")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	n, err := svc.GetNote(context.Background(), "source.md")
	require.NoError(t, err)

	// existing links field is untouched: normalized targets, order preserved.
	require.Equal(t, []string{"target", "ghost"}, n.Links)

	// link_targets resolves each wikilink to its vault-relative path; empty when broken.
	require.Len(t, n.LinkTargets, 2)
	require.Equal(t, "target", n.LinkTargets[0].Link)
	require.Equal(t, "notes/target.md", n.LinkTargets[0].Path) // resolved across folders
	require.Equal(t, "ghost", n.LinkTargets[1].Link)
	require.Equal(t, "", n.LinkTargets[1].Path) // unresolved stays empty
}

func TestGetNoteNoLinks(t *testing.T) {
	root := emptyVault(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "lonely.md"),
		[]byte("---\ntitle: Lonely\n---\n# Lonely\nno links here"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	n, err := svc.GetNote(context.Background(), "lonely.md")
	require.NoError(t, err)
	require.Empty(t, n.Links)
	require.Empty(t, n.LinkTargets) // no links -> empty, non-nil slice
}

func TestGraphReportIncludesPageRank(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	write("hub.md", "---\ntitle: Hub\n---\n# Hub\nsee [[a]]")
	write("a.md", "---\ntitle: A\n---\n# A\nsee [[hub]]")
	write("b.md", "---\ntitle: B\n---\n# B\nsee [[hub]]")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	rep, err := svc.Graph(context.Background())
	require.NoError(t, err)

	// existing fields unchanged.
	require.Equal(t, 3, rep.Notes)
	require.GreaterOrEqual(t, rep.Links, 1)

	// new pagerank field: the hub ranks first by centrality.
	require.NotEmpty(t, rep.PageRank)
	require.Equal(t, "hub.md", rep.PageRank[0].Path)
	require.Equal(t, "Hub", rep.PageRank[0].Title)
	require.Greater(t, rep.PageRank[0].Score, 0.0)
}
