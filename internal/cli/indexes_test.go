package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
)

func indexesRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	cfg := config.Default()
	cfg.Conventions.DirectoryIndexes = config.DirectoryIndexesConfig{
		Enabled: true,
		Roots:   []string{"20-Profile"},
	}
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), cfg))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "20-Profile"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "20-Profile", "profile.md"), []byte("# Profile\n"), 0o644))
	return root
}

func TestIndexesCmdSyncsDirectoryIndexes(t *testing.T) {
	root := indexesRepo(t)
	t.Setenv("STARDUST_VAULT", root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"indexes", "--output", "plain"})
	require.NoError(t, cmd.Execute())

	body, err := os.ReadFile(filepath.Join(root, "20-Profile", "INDEX.md"))
	require.NoError(t, err)
	require.Contains(t, string(body), "stardust-directory-index:start")
	require.Contains(t, out.String(), "Directory indexes")
}

func TestIndexesCmdCheckReportsIssues(t *testing.T) {
	root := indexesRepo(t)
	t.Setenv("STARDUST_VAULT", root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"indexes", "--check", "--output", "plain"})
	require.Error(t, cmd.Execute())
	require.Contains(t, out.String(), "directory-index-missing")
}
