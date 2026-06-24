package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckStrictFailsForConventionErrors(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, scaffoldVault(t.Context(), root, "off", false))
	t.Setenv("STARDUST_VAULT", root)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "specs", "bad-name.md"), []byte("---\ntitle: Bad\ntype: spec\nstatus: Weird\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Bad\n"), 0o644))
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"check", "--strict", "--output", "plain"})
	require.Error(t, cmd.Execute())
	require.Contains(t, out.String(), "bad-doc-status")
}

func TestCheckFixRewritesBadType(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, scaffoldVault(t.Context(), root, "off", false))
	t.Setenv("STARDUST_VAULT", root)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-bad-type.md")
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("---\ntitle: Bad\ntype: plan\nstatus: Draft\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Bad\n"), 0o644))
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"check", "--fix", "--output", "plain"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "bad-doc-type")

	fixed, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err)
	require.Contains(t, string(fixed), "type: spec")
}

func TestCheckFixThenStrictPasses(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, scaffoldVault(t.Context(), root, "off", false))
	t.Setenv("STARDUST_VAULT", root)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-dashy.md")
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("---\ntitle: Dashy\ntype: spec\nstatus: Draft\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Dashy\n\nfoo — bar\n"), 0o644))
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"check", "--fix", "--strict", "--output", "plain"})
	require.NoError(t, cmd.Execute())

	fixed, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err)
	require.NotContains(t, string(fixed), "—")
}
