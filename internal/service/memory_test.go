package service_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

func emptyVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	return root
}

func TestRememberCreatesAndIndexes(t *testing.T) {
	root := emptyVault(t)
	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	// empty vault -> nothing to dedup into -> creates a new memory note
	res, err := svc.Remember(context.Background(), "the deploy key rotates every 90 days")
	require.NoError(t, err)
	require.Equal(t, "created", res.Action)
	require.True(t, strings.HasPrefix(res.Path, "memory/"))

	// the new note exists on disk and was indexed
	_, statErr := os.Stat(filepath.Join(root, res.Path))
	require.NoError(t, statErr)
	st, err := svc.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, st.Notes)
}

func TestMemoryVerbsViaService(t *testing.T) {
	root := emptyVault(t)
	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	ctx := context.Background()
	_, err = svc.Memory(ctx, service.MemoryOp{Command: "create", Path: "notes/x.md", Content: "---\ntitle: X\n---\n# X\nalpha content"})
	require.NoError(t, err)

	st, _ := svc.Status(ctx)
	require.Equal(t, 1, st.Notes) // create reindexed the note

	out, err := svc.Memory(ctx, service.MemoryOp{Command: "view", Path: "notes/x.md"})
	require.NoError(t, err)
	require.Contains(t, out, "alpha content")

	_, err = svc.Memory(ctx, service.MemoryOp{Command: "str_replace", Path: "notes/x.md", Old: "alpha", New: "beta"})
	require.NoError(t, err)
	out, _ = svc.Memory(ctx, service.MemoryOp{Command: "view", Path: "notes/x.md"})
	require.Contains(t, out, "beta content")
}
