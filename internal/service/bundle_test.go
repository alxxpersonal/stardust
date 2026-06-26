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

	// the bundle inherits the query's retrieval mode; with no Ollama it is fts-only.
	require.Equal(t, "fts-only", res.RetrievalMode)
	require.NotEmpty(t, res.RetrievalReason)
}

func TestBundleProvenance(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	// kubernetes <-> pods are wikilinked both ways; pods references sidecar via related: only.
	write("kubernetes.md", "---\ntitle: Kubernetes\n---\n# Kubernetes\nDeployments manage pods and rolling updates. See [[pods]].")
	write("pods.md", "---\ntitle: Pods\nrelated: [\"sidecar.md\"]\n---\n# Pods\nA pod is the smallest deployable unit. See [[kubernetes]].")
	write("sidecar.md", "---\ntitle: Sidecar\n---\n# Sidecar\nA helper container pattern that runs beside the main one.")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	res, err := svc.Bundle(context.Background(), "kubernetes pods deployment", 2000)
	require.NoError(t, err)

	byPath := map[string]service.BundleItem{}
	for _, it := range res.Items {
		byPath[it.Path] = it
	}

	// kubernetes is both an FTS hit and a wikilink neighbor: keyword + link-expansion.
	k, ok := byPath["kubernetes.md"]
	require.True(t, ok)
	require.Contains(t, k.Provenance, "keyword")
	require.Contains(t, k.Provenance, "link-expansion")

	// sidecar is reached only through pods' related: edge: frontmatter-ref, not keyword.
	s, ok := byPath["sidecar.md"]
	require.True(t, ok)
	require.Contains(t, s.Provenance, "frontmatter-ref")
	require.NotContains(t, s.Provenance, "keyword")

	// provenance is rendered inline in the pack.
	require.Contains(t, res.Markdown, "frontmatter-ref")
}

func TestBundleFreshnessStamp(t *testing.T) {
	root := gitInitVault(t)
	write := func(name, content string) {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write("kubernetes.md", "---\ntitle: Kubernetes\n---\n# Kubernetes\nDeployments manage pods and rolling updates.")
	gitCommitAt(t, root, "2026-01-01T00:00:00", "add notes")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	// the index sits at HEAD: zero commits behind.
	res0, err := svc.Bundle(context.Background(), "kubernetes deployment", 2000)
	require.NoError(t, err)
	require.Equal(t, 0, res0.CommitsBehind)
	require.Contains(t, res0.Markdown, "index is 0 commits behind HEAD")

	// advance HEAD by two commits without reindexing.
	write("kubernetes.md", "---\ntitle: Kubernetes\n---\n# Kubernetes\nDeployments manage pods. Edit one.")
	gitCommitAt(t, root, "2026-01-02T00:00:00", "edit 1")
	write("kubernetes.md", "---\ntitle: Kubernetes\n---\n# Kubernetes\nDeployments manage pods. Edit two.")
	gitCommitAt(t, root, "2026-01-03T00:00:00", "edit 2")

	res, err := svc.Bundle(context.Background(), "kubernetes deployment", 2000)
	require.NoError(t, err)
	require.Equal(t, 2, res.CommitsBehind)
	require.Contains(t, res.Markdown, "index is 2 commits behind HEAD")
}

// TestBundleSurfacesDrift asserts a bundled doc that references moved code
// carries the drift binding on its item and renders it in the pack as a review
// prompt.
func TestBundleSurfacesDrift(t *testing.T) {
	root := gitInitVaultBare(t)
	writeGovernedCode(t, root, "internal/store/daemon.go")
	writeReferencingDoc(t, root, "docs/adr/0001-daemon.md", "Daemon ADR", "adr", "Proposed", "internal/store/daemon.go")
	gitCommitAt(t, root, "2026-01-01T00:00:00", "add adr and code")

	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "store", "daemon.go"), []byte("package store\n\nconst X = 1\n"), 0o644))
	gitCommitAt(t, root, "2026-01-02T00:00:00", "edit daemon")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	res, err := svc.Bundle(context.Background(), "daemon store", 2000)
	require.NoError(t, err)

	var adr *service.BundleItem
	for i := range res.Items {
		if res.Items[i].Path == "docs/adr/0001-daemon.md" {
			adr = &res.Items[i]
			break
		}
	}
	require.NotNil(t, adr, "expected the adr in the bundle, got %#v", res.Items)
	require.NotEmpty(t, adr.Drift)
	require.Equal(t, "internal/store/daemon.go", adr.Drift[0].File)
	require.Greater(t, adr.Drift[0].ChangedCommits, 0)

	require.Contains(t, res.Markdown, "internal/store/daemon.go")
	require.Contains(t, res.Markdown, "review")
}

// gitInitVaultBare initializes a git vault with stardust config and cache but no
// scaffolded notes, for tests that author their own files.
func gitInitVaultBare(t *testing.T) string {
	t.Helper()
	root := emptyVault(t)
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "t@t")
	gitRun(t, root, "config", "user.name", "t")
	gitRun(t, root, "config", "commit.gpgsign", "false")
	return root
}
