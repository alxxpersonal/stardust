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
	require.Empty(t, st.Source.Path)
	require.Empty(t, st.Source.Origin)
}

// TestGatherStatusConfiguredSourceRoot asserts an explicit source_root surfaces
// on the status report as a configured binding.
func TestGatherStatusConfiguredSourceRoot(t *testing.T) {
	dir := t.TempDir()
	layout := config.Layout{Root: dir}
	require.NoError(t, os.MkdirAll(layout.Cache(), 0o755))
	cfg := config.Default()
	cfg.SourceRoot = filepath.Join(dir, "explicit-source")
	require.NoError(t, config.Save(layout.Config(), cfg))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644))

	st, err := GatherStatus(context.Background(), dir)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "explicit-source"), st.Source.Path)
	require.Equal(t, "configured", st.Source.Origin)
}

// TestGatherStatusDetectedSourceRoot asserts a <name>.wiki vault with a same-repo
// sibling checkout surfaces the sibling as a detected binding.
func TestGatherStatusDetectedSourceRoot(t *testing.T) {
	parent := t.TempDir()
	wikiRoot := filepath.Join(parent, "project.wiki")
	layout := config.Layout{Root: wikiRoot}
	require.NoError(t, os.MkdirAll(layout.Cache(), 0o755))
	require.NoError(t, config.Save(layout.Config(), config.Default()))
	writeStatusGitRemote(t, wikiRoot, "https://github.com/acme/project.wiki.git")

	src := filepath.Join(parent, "project")
	require.NoError(t, os.MkdirAll(src, 0o755))
	writeStatusGitRemote(t, src, "https://github.com/acme/project.git")

	st, err := GatherStatus(context.Background(), wikiRoot)
	require.NoError(t, err)
	require.Equal(t, filepath.Clean(src), st.Source.Path)
	require.Equal(t, "detected", st.Source.Origin)
}

func writeStatusGitRemote(t *testing.T, dir, url string) {
	t.Helper()
	cfgPath := filepath.Join(dir, ".git", "config")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o755))
	content := "[remote \"origin\"]\n\turl = " + url + "\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0o644))
}
