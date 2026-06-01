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

func TestBundleAssembles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	write("kubernetes.md", "---\ntitle: Kubernetes\n---\n# Kubernetes\nDeployments manage pods and rolling updates. See [[pods]].")
	write("pods.md", "---\ntitle: Pods\n---\n# Pods\nA pod is the smallest deployable unit. Related to [[kubernetes]].")
	write("cooking.md", "---\ntitle: Cooking\n---\n# Cooking\nCarbonara needs guanciale and pecorino.")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	res, err := svc.Bundle(context.Background(), "kubernetes deployment pods", 2000)
	require.NoError(t, err)
	require.Contains(t, res.Markdown, "## Task")
	require.Contains(t, res.Markdown, "kubernetes deployment pods")
	require.NotEmpty(t, res.Items)
	require.NotEqual(t, "cooking.md", res.Items[0].Path) // a k8s note should outrank cooking
	require.LessOrEqual(t, res.Tokens, 2000)             // respects the budget
}
