package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
)

// TestGatherStatusUninitialized asserts that a directory with no .stardust
// reports an uninitialized status (with detected kind, a hint, and a non-nil
// empty collections slice) and a nil error.
func TestGatherStatusUninitialized(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".obsidian"), 0o755))

	st, err := GatherStatus(context.Background(), dir)
	require.NoError(t, err)
	require.False(t, st.Initialized)
	require.Equal(t, "plain-vault", st.Kind)
	require.NotEmpty(t, st.Hint)
	require.NotNil(t, st.Collections)
	require.Empty(t, st.Collections)
}

// TestGatherStatusInitialized builds a minimal initialized vault that also looks
// like a code repo (a go.mod at the root) and asserts the composed report:
// initialized, the code-repo kind, and a zero note count for the empty index.
func TestGatherStatusInitialized(t *testing.T) {
	dir := t.TempDir()
	layout := config.Layout{Root: dir}
	require.NoError(t, os.MkdirAll(layout.Cache(), 0o755))
	require.NoError(t, config.Save(layout.Config(), config.Default()))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644))

	st, err := GatherStatus(context.Background(), dir)
	require.NoError(t, err)
	require.True(t, st.Initialized)
	require.Equal(t, "code-repo-with-docs", st.Kind)
	require.Equal(t, 0, st.Index.Notes)
	require.NotNil(t, st.Collections)
}
