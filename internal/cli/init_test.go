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

// TestInitDocsScaffoldsAgentsArea runs `init --docs` and asserts the docs/agents
// home is scaffolded: skills/ and subagents/ subfolders plus a rules.md stub.
func TestInitDocsScaffoldsAgentsArea(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--docs"})
	require.NoError(t, cmd.Execute())

	for _, sub := range []string{"skills", "subagents"} {
		info, err := os.Stat(filepath.Join(root, "docs", "agents", sub))
		require.NoErrorf(t, err, "expected docs/agents/%s", sub)
		require.Truef(t, info.IsDir(), "docs/agents/%s must be a directory", sub)
	}
	rules := filepath.Join(root, "docs", "agents", "rules.md")
	info, err := os.Stat(rules)
	require.NoError(t, err)
	require.False(t, info.IsDir(), "docs/agents/rules.md must be a file")
	body, err := os.ReadFile(rules)
	require.NoError(t, err)
	require.NotEmpty(t, body, "rules.md stub must have content")
}

// TestInitDocsScaffoldsConfiguredAgentsDir asserts init --docs scaffolds the
// agents area at a pre-existing config's agents_dir (docs/skills) instead of the
// default docs/agents.
func TestInitDocsScaffoldsConfiguredAgentsDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust"), 0o755))
	cfg := config.Default()
	cfg.AgentsDirRaw = "docs/skills"
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), cfg))
	t.Chdir(root)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--docs"})
	require.NoError(t, cmd.Execute())

	for _, sub := range []string{"skills", "subagents"} {
		info, err := os.Stat(filepath.Join(root, "docs", "skills", sub))
		require.NoErrorf(t, err, "expected docs/skills/%s", sub)
		require.Truef(t, info.IsDir(), "docs/skills/%s must be a directory", sub)
	}
	_, err := os.Stat(filepath.Join(root, "docs", "skills", "rules.md"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, "docs", "agents"))
	require.True(t, os.IsNotExist(err), "relocated config must not scaffold docs/agents")
}

// TestInitNoDocsSkipsAgentsArea asserts a plain `init` does not scaffold docs/agents.
func TestInitNoDocsSkipsAgentsArea(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	cmd := newInitCmd()
	require.NoError(t, cmd.Execute())

	_, err := os.Stat(filepath.Join(root, "docs", "agents"))
	require.True(t, os.IsNotExist(err), "plain init must not scaffold docs/agents")
}

// TestInitDocsScaffoldAgentsAreaIdempotent asserts a second `init --docs` never
// clobbers an edited rules.md.
func TestInitDocsScaffoldAgentsAreaIdempotent(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--docs"})
	require.NoError(t, cmd.Execute())

	rules := filepath.Join(root, "docs", "agents", "rules.md")
	require.NoError(t, os.WriteFile(rules, []byte("# My real rules\n"), 0o644))

	cmd2 := newInitCmd()
	cmd2.SetArgs([]string{"--docs"})
	require.NoError(t, cmd2.Execute())

	body, err := os.ReadFile(rules)
	require.NoError(t, err)
	require.Equal(t, "# My real rules\n", string(body), "re-running init must not clobber edited rules.md")
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
