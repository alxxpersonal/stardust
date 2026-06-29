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

func directoryIndexVault(t *testing.T) string {
	t.Helper()
	root := emptyVault(t)
	cfg, err := config.Load(config.Layout{Root: root}.Config())
	require.NoError(t, err)
	cfg.Conventions.DirectoryIndexes = config.DirectoryIndexesConfig{
		Enabled: true,
		Roots:   []string{"20-Profile"},
		Ignore:  []string{"skip-me"},
	}
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), cfg))
	write := func(rel, body string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	}
	write("20-Profile/2026-06-29-profile.md", "---\ntitle: Profile Copy\n---\n# Profile Copy\n")
	write("20-Profile/portfolio.md", "# Portfolio\n")
	write("20-Profile/proposals/2026-06-28-sample.md", "# Sample Proposal\n")
	write("20-Profile/skip-me/ignored.md", "# Ignored\n")
	write("20-Profile/notes.txt", "notes\n")
	return root
}

func TestSyncDirectoryIndexesCreatesAndUpdatesManagedBlocks(t *testing.T) {
	ctx := context.Background()
	root := directoryIndexVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.SyncDirectoryIndexes(ctx)
	require.NoError(t, err)
	require.True(t, res.Enabled)
	require.Len(t, res.Files, 2)

	rootIndex := filepath.Join(root, "20-Profile", "INDEX.md")
	body, err := os.ReadFile(rootIndex)
	require.NoError(t, err)
	got := string(body)
	require.Contains(t, got, "# 20-Profile Index")
	require.Contains(t, got, "<!-- stardust-directory-index:start -->")
	require.Contains(t, got, "[2026-06-29-profile.md](2026-06-29-profile.md) | Profile Copy.")
	require.Contains(t, got, "[portfolio.md](portfolio.md) | Portfolio.")
	require.Contains(t, got, "[proposals](proposals/)")
	require.NotContains(t, got, "skip-me")

	res2, err := svc.SyncDirectoryIndexes(ctx)
	require.NoError(t, err)
	for _, file := range res2.Files {
		require.False(t, file.Updated)
	}
}

func TestSyncDirectoryIndexesPreservesPreamble(t *testing.T) {
	ctx := context.Background()
	root := directoryIndexVault(t)
	indexPath := filepath.Join(root, "20-Profile", "INDEX.md")
	require.NoError(t, os.WriteFile(indexPath, []byte("# Custom Index\n\nHuman text.\n"), 0o644))
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	_, err = svc.SyncDirectoryIndexes(ctx)
	require.NoError(t, err)
	body, err := os.ReadFile(indexPath)
	require.NoError(t, err)
	require.Contains(t, string(body), "# Custom Index\n\nHuman text.")
	require.Contains(t, string(body), "<!-- stardust-directory-index:start -->")
}

func TestCheckDirectoryIndexesReportsMissingAndStale(t *testing.T) {
	ctx := context.Background()
	root := directoryIndexVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	check, err := svc.CheckDirectoryIndexes(ctx)
	require.NoError(t, err)
	require.Len(t, check.Issues, 2)
	require.Equal(t, "directory-index-missing", check.Issues[0].Kind)

	_, err = svc.SyncDirectoryIndexes(ctx)
	require.NoError(t, err)
	check, err = svc.CheckDirectoryIndexes(ctx)
	require.NoError(t, err)
	require.Empty(t, check.Issues)

	require.NoError(t, os.WriteFile(filepath.Join(root, "20-Profile", "new.md"), []byte("# New\n"), 0o644))
	check, err = svc.CheckDirectoryIndexes(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, check.Issues)
	require.Equal(t, "directory-index-stale", check.Issues[0].Kind)
}
