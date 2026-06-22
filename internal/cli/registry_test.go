package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

// docsRepo builds a temp vault with a "specs" collection mapped at docs/specs
// and one timestamped sample doc carrying title/status frontmatter, indexes it,
// and returns the vault root. It mirrors the service package test harness.
func docsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))

	dir := filepath.Join(root, ".stardust", "collections", "specs")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	cfg := "path = \"docs/specs\"\n" +
		"description = \"docs specs\"\n\n" +
		"[[fields]]\nname = \"title\"\ntype = \"string\"\nrequired = true\n\n" +
		"[[fields]]\nname = \"status\"\ntype = \"enum\"\nenum = [\"Draft\", \"Approved\"]\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0o644))

	specsDir := filepath.Join(root, "docs", "specs")
	require.NoError(t, os.MkdirAll(specsDir, 0o755))
	note := "---\ntitle: First Spec\nstatus: Approved\n---\n\n# First Spec\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "2026-06-22-2238-first-spec.md"), []byte(note), 0o644))

	ctx := t.Context()
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	return root
}

func TestRegistryCmd(t *testing.T) {
	root := docsRepo(t)
	t.Setenv("STARDUST_VAULT", root)

	cmd := newRegistryCmd()
	cmd.SetArgs([]string{"--output", "docs/INDEX.md"})
	require.NoError(t, cmd.Execute())

	out := filepath.Join(root, "docs", "INDEX.md")
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	got := string(data)

	require.Contains(t, got, "# Docs Index")
	require.Contains(t, got, "## Specs")
	require.Contains(t, got, "| Title | Status | Doc | Date |")
	require.Contains(t, got, "| First Spec | Approved | docs/specs/2026-06-22-2238-first-spec.md | 2026-06-22 |")

	// Idempotent: a second run produces byte-identical output.
	cmd2 := newRegistryCmd()
	cmd2.SetArgs([]string{"--output", "docs/INDEX.md"})
	require.NoError(t, cmd2.Execute())
	data2, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Equal(t, data, data2)
}
