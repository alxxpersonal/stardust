package cli

import (
	"bytes"
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

// specsConfigPath is the scaffolded specs collection config under a vault root.
func specsConfigPath(root string) string {
	return filepath.Join(root, ".stardust", "collections", "specs", "config.toml")
}

// TestInitAutoDetectsCodeRepo asserts that with neither flag a go.mod+.git dir
// is detected as a code repo, scaffolds the docs collections, and prints the
// detection line.
func TestInitAutoDetectsCodeRepo(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	t.Chdir(root)

	var buf bytes.Buffer
	cmd := newInitCmd()
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())

	_, err := os.Stat(specsConfigPath(root))
	require.NoError(t, err, "code repo auto-detect must scaffold docs collections")
	require.Contains(t, buf.String(), "detected a code repo")
}

// TestInitAutoDetectsPlainVault asserts that with neither flag an .obsidian dir
// is detected as a plain vault, skips the docs collections, and prints the
// detection line.
func TestInitAutoDetectsPlainVault(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".obsidian"), 0o755))
	t.Chdir(root)

	var buf bytes.Buffer
	cmd := newInitCmd()
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())

	_, err := os.Stat(specsConfigPath(root))
	require.True(t, os.IsNotExist(err), "plain vault auto-detect must not scaffold docs collections")
	require.Contains(t, buf.String(), "detected a plain vault")
}

// TestInitNoDocsOverridesCodeRepo asserts --no-docs skips the docs collections
// even when the directory looks like a code repo.
func TestInitNoDocsOverridesCodeRepo(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	t.Chdir(root)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--no-docs"})
	require.NoError(t, cmd.Execute())

	_, err := os.Stat(specsConfigPath(root))
	require.True(t, os.IsNotExist(err), "--no-docs must override the code-repo default")
}

// TestInitDocsOverridesPlainVault asserts --docs scaffolds the docs collections
// even when the directory looks like a plain vault.
func TestInitDocsOverridesPlainVault(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".obsidian"), 0o755))
	t.Chdir(root)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--docs"})
	require.NoError(t, cmd.Execute())

	_, err := os.Stat(specsConfigPath(root))
	require.NoError(t, err, "--docs must override the plain-vault default")
}
