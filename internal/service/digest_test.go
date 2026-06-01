package service_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

func gitInit(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
		{"add", "-A"}, {"commit", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		require.NoError(t, cmd.Run(), "git %v", args)
	}
}

func TestDigest(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.WriteFile(filepath.Join(root, "todo.md"),
		[]byte("---\ntitle: Todo\n---\n# Todo\nTODO: ship the digest feature.\nI'll review the change."), 0o644))
	gitInit(t, root)

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Digest(context.Background(), "", false)
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Changed, 1)
	require.Contains(t, res.Markdown, "# Digest")
	require.Contains(t, res.Markdown, "todo.md")
	require.Contains(t, res.Markdown, "Open commitments")
	require.Contains(t, res.Markdown, "ship the digest feature") // commitment surfaced
}
