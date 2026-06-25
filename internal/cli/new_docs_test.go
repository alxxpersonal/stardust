package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDocSpecCmdCreatesDoc(t *testing.T) {
	root := t.TempDir()
	_, err := scaffoldVault(t.Context(), root, "off", true)
	require.NoError(t, err)
	t.Setenv("STARDUST_VAULT", root)
	t.Chdir(root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"new", "spec", "Agent Infra", "--status", "Draft", "--governs", "internal/*.go"})
	require.NoError(t, cmd.Execute())

	matches, err := filepath.Glob(filepath.Join(root, "docs", "specs", "*-agent-infra.md"))
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Contains(t, out.String(), "docs/specs/")
}

func TestNewVaultCommandStillScaffoldsVault(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "vault")
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"new", target, "--check", "off"})
	require.NoError(t, cmd.Execute())

	_, err := os.Stat(filepath.Join(target, ".stardust", "config.toml"))
	require.NoError(t, err)
	require.Contains(t, out.String(), "Created vault")
}
