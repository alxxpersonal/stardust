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
