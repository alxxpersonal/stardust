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

func TestCheckFindsIssues(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	write("a.md", "---\ntitle: A\n---\n# A\nlinks to [[nonexistent]]") // broken link (error)
	write("b.md", "---\ntitle: [bad\n---\n# B\n")                      // bad frontmatter (error)
	write("orphan.md", "---\ntitle: Orphan\n---\n# Orphan\nno links")  // orphan (warn)

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Errors, 2) // broken link + bad frontmatter
	require.Greater(t, res.Warnings, 0)      // orphans

	var foundBroken, foundBadFM bool
	for _, is := range res.Issues {
		switch is.Kind {
		case "broken-link":
			foundBroken = true
		case "bad-frontmatter":
			foundBadFM = true
		}
	}
	require.True(t, foundBroken)
	require.True(t, foundBadFM)
}

func TestCheckCleanVault(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.WriteFile(filepath.Join(root, "x.md"), []byte("---\ntitle: X\n---\n# X\nsee [[y]]"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "y.md"), []byte("---\ntitle: Y\n---\n# Y\nsee [[x]]"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, res.Errors)
	require.Equal(t, 0, res.Warnings) // both linked, both titled, no dupes
}
