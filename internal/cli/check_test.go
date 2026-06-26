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
	_, err := scaffoldVault(t.Context(), root, "off", true)
	require.NoError(t, err)
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
	_, err := scaffoldVault(t.Context(), root, "off", true)
	require.NoError(t, err)
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

func TestCheckCIRatchetExit(t *testing.T) {
	root := t.TempDir()
	_, err := scaffoldVault(t.Context(), root, "off", true)
	require.NoError(t, err)
	t.Setenv("STARDUST_VAULT", root)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "specs", "2026-06-22-1000-one.md"), []byte("---\ntitle: One\ntype: spec\nstatus: Weird\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# One\n"), 0o644))

	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := newRootCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	defer func() { _, _ = run() }()

	// snapshot the existing backlog, then the ratchet adopts green.
	_, err = run("check", "--update-baseline", "--output", "plain")
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, ".stardust", "baseline.json"))
	_, err = run("check", "--ci", "--output", "plain")
	require.NoError(t, err)

	// a brand-new error fails the gate and names exactly the new issue.
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "specs", "2026-06-22-1000-two.md"), []byte("---\ntitle: Two\ntype: spec\nstatus: AlsoWeird\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Two\n"), 0o644))
	out, err := run("check", "--ci", "--output", "plain")
	require.Error(t, err)
	require.Contains(t, out, "bad-doc-status")
	require.Contains(t, out, "2026-06-22-1000-two.md")
}

func TestCheckFixThenStrictPasses(t *testing.T) {
	root := t.TempDir()
	_, err := scaffoldVault(t.Context(), root, "off", true)
	require.NoError(t, err)
	t.Setenv("STARDUST_VAULT", root)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-dashy.md")
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("---\ntitle: Dashy\ntype: spec\nstatus: Draft\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Dashy\n\nfoo \u2014 bar\n"), 0o644))
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"check", "--fix", "--strict", "--output", "plain"})
	require.NoError(t, cmd.Execute())

	fixed, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err)
	require.NotContains(t, string(fixed), "\u2014")
}
