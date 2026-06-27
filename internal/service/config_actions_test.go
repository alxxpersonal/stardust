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

func TestConfigActionsSetConfig(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	cfg := config.Default()
	cfg.EmbedModel = "nomic-embed-text"
	cfg.OllamaURL = "http://localhost:9999"
	require.NoError(t, svc.SetConfig(cfg))

	// Persisted to disk.
	onDisk, err := config.Load(config.Layout{Root: root}.Config())
	require.NoError(t, err)
	require.Equal(t, "nomic-embed-text", onDisk.EmbedModel)
	require.Equal(t, "http://localhost:9999", onDisk.OllamaURL)

	// Reflected on the live service: no reindex, so the meta is empty and Status
	// falls back to the rebuilt embed client's model.
	require.Equal(t, "nomic-embed-text", svc.Config.EmbedModel)
	st, err := svc.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, "nomic-embed-text", st.EmbedModel)
}

func TestConfigActionsRegenerateRegistry(t *testing.T) {
	ctx := context.Background()
	root := docsVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	require.NoError(t, svc.RegenerateRegistry(ctx))

	out := filepath.Join(root, "docs", "INDEX.md")
	info, err := os.Stat(out)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))

	body, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Contains(t, string(body), "Docs Index")
	require.Contains(t, string(body), "Second Spec")

	// The pinned agent manifest is refreshed alongside the registry.
	_, err = os.Stat(config.Layout{Root: root}.Manifest())
	require.NoError(t, err)
}
