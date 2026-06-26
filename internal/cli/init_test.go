package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/convention"
)

// TestDocCollectionConfigCodegensAllFields asserts the scaffolder codegens every
// schema field declared by Fields(), and that the emitted TOML round-trips
// through collections.LoadOne back to the same field set.
func TestDocCollectionConfigCodegensAllFields(t *testing.T) {
	dir := t.TempDir()
	for _, c := range convention.DefaultDocCollections() {
		cdir := filepath.Join(dir, c.Name)
		require.NoError(t, os.MkdirAll(cdir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(cdir, "config.toml"), []byte(docCollectionConfig(c)), 0o644))

		col, err := collections.LoadOne(dir, c.Name)
		require.NoErrorf(t, err, "LoadOne(%s)", c.Name)
		require.Equal(t, c.Path, col.Cfg.Path)
		require.Equal(t, c.Description, col.Cfg.Description)
		require.Equal(t, c.Fields(), col.Cfg.Fields)
	}
}

// TestInitDocsScaffold runs `init --docs` in a temp vault and asserts the four
// docs collection configs are written under .stardust/collections/.
func TestInitDocsScaffold(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--docs"})
	require.NoError(t, cmd.Execute())

	for _, name := range []string{"specs", "plans", "adr", "research"} {
		cfg := filepath.Join(root, ".stardust", "collections", name, "config.toml")
		_, err := os.Stat(cfg)
		require.NoErrorf(t, err, "expected config for collection %s", name)
	}
	_, err := os.Stat(config.Layout{Root: root}.Manifest())
	require.NoError(t, err)
}

// TestInitNoDocs runs a plain `init` and asserts no docs collections are written.
func TestInitNoDocs(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	cmd := newInitCmd()
	require.NoError(t, cmd.Execute())

	_, err := os.Stat(filepath.Join(root, ".stardust", "collections", "specs", "config.toml"))
	require.True(t, os.IsNotExist(err), "plain init must not scaffold docs collections")
}
