package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/index"
)

// TestIndexAndQueryNonMarkdownPages proves a mixed-format wiki vault indexes its
// non-markdown pages, that a term living only in a non-markdown body is
// searchable, that Tier A titles survive to the hit, and that a no-change
// reindex skips every page by content hash.
func TestIndexAndQueryNonMarkdownPages(t *testing.T) {
	svc, root := newServiceWith(t, &fakeEmbedder{available: true}, "")
	ctx := context.Background()
	write := func(name, content string) {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}

	write("Home.md", "# Home\n\nsee [[Install]]")
	write("Install.rst", "Install Guide\n=============\n\nRun the zorptastic installer to bootstrap the widget.\n")
	write("Guide.adoc", "= Admin Guide\n\nConfigure the frobnicator via the control panel.\n")
	write("Style.textile", "h1. Style Rules\n\nUse the quazzle spacing token everywhere.\n")
	write("Notes.org", "#+TITLE: Release Notes\n\nThe plindrome milestone shipped on schedule.\n")

	stats, err := svc.Index(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 5, stats.Indexed)

	catalog, err := svc.store.Catalog(ctx)
	require.NoError(t, err)
	for _, p := range []string{"Home.md", "Install.rst", "Guide.adoc", "Style.textile", "Notes.org"} {
		require.Contains(t, catalog, p)
	}

	// a rare term that exists only in the .rst body is searchable and carries the
	// Tier A title extracted from the RST heading heuristic.
	res, err := svc.Query(ctx, "zorptastic", 5)
	require.NoError(t, err)
	hit, ok := findHit(res.Hits, "Install.rst")
	require.True(t, ok, "expected a hit from Install.rst, got %#v", res.Hits)
	require.Equal(t, "Install Guide", hit.Title)

	// terms unique to the other non-markdown formats are each retrievable.
	for term, wantPath := range map[string]string{
		"frobnicator": "Guide.adoc",
		"quazzle":     "Style.textile",
		"plindrome":   "Notes.org",
	} {
		r, err := svc.Query(ctx, term, 5)
		require.NoError(t, err)
		_, found := findHit(r.Hits, wantPath)
		require.True(t, found, "term %q should retrieve %s, got %#v", term, wantPath, r.Hits)
	}

	// a no-change reindex skips every page by content hash.
	stats2, err := svc.Index(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 0, stats2.Indexed)
	require.Equal(t, 5, stats2.Skipped)
}

func findHit(hits []index.Hit, path string) (index.Hit, bool) {
	for _, h := range hits {
		if h.Path == path {
			return h, true
		}
	}
	return index.Hit{}, false
}
