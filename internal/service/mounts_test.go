package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

func TestMountsListsConfigured(t *testing.T) {
	root := emptyVault(t)
	mountsDir := config.Layout{Root: root}.Mounts()
	require.NoError(t, os.MkdirAll(filepath.Join(mountsDir, "gmail"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mountsDir, "gmail", "config.toml"),
		[]byte("command = \"gmail-mcp\"\nargs = [\"serve\"]\ntool = \"search\"\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(mountsDir, "zotero"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mountsDir, "zotero", "config.toml"),
		[]byte("command = \"zotero-mcp\"\n"), 0o644)) // tool omitted -> defaults to "query"

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	ms, err := svc.Mounts()
	require.NoError(t, err)
	require.Len(t, ms, 2)

	// mounts.Load sorts by name: gmail then zotero.
	require.Equal(t, "gmail", ms[0].Name)
	require.Equal(t, "mcp", ms[0].Kind)
	require.Equal(t, "gmail-mcp", ms[0].Target)
	require.Equal(t, []string{"serve"}, ms[0].Args)
	require.Equal(t, "search", ms[0].Tool)

	require.Equal(t, "zotero", ms[1].Name)
	require.Equal(t, "zotero-mcp", ms[1].Target)
	require.Equal(t, "query", ms[1].Tool) // defaulted
}

func TestMountsEmptyVault(t *testing.T) {
	root := emptyVault(t)
	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	ms, err := svc.Mounts()
	require.NoError(t, err)
	require.Empty(t, ms) // no mounts dir -> empty slice, no error
}
