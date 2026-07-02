package convention

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
)

// TestResolveSourceRootConfiguredWins asserts an explicit source_root is returned
// verbatim with origin configured and never triggers sibling probing, even when a
// confirmable sibling exists and even when the configured path does not exist.
func TestResolveSourceRootConfiguredWins(t *testing.T) {
	parent := t.TempDir()
	wiki := filepath.Join(parent, "project.wiki")
	require.NoError(t, os.MkdirAll(wiki, 0o755))
	writeSourceRootGitRemote(t, wiki, "https://github.com/acme/project.wiki.git")
	src := filepath.Join(parent, "project")
	require.NoError(t, os.MkdirAll(src, 0o755))
	writeSourceRootGitRemote(t, src, "https://github.com/acme/project.git")

	explicit := filepath.Join(parent, "does-not-exist")
	cfg := config.Default()
	cfg.SourceRoot = explicit

	path, origin, err := ResolveSourceRoot(cfg, wiki)
	require.NoError(t, err)
	require.Equal(t, filepath.Clean(explicit), path)
	require.Equal(t, SourceOriginConfigured, origin)
}

// TestResolveSourceRootDetectsSibling asserts a <name>.wiki workspace with a
// same-repo sibling ../<name> checkout resolves to the sibling with origin
// detected when source_root is unset.
func TestResolveSourceRootDetectsSibling(t *testing.T) {
	parent := t.TempDir()
	wiki := filepath.Join(parent, "project.wiki")
	require.NoError(t, os.MkdirAll(wiki, 0o755))
	writeSourceRootGitRemote(t, wiki, "https://github.com/acme/project.wiki.git")
	src := filepath.Join(parent, "project")
	require.NoError(t, os.MkdirAll(src, 0o755))
	writeSourceRootGitRemote(t, src, "git@github.com:acme/project.git")

	path, origin, err := ResolveSourceRoot(config.Default(), wiki)
	require.NoError(t, err)
	require.Equal(t, filepath.Clean(src), path)
	require.Equal(t, SourceOriginDetected, origin)
}

// TestResolveSourceRootBindsNothing asserts every single missing condition binds
// nothing: an empty path and empty origin, byte-identical to today.
func TestResolveSourceRootBindsNothing(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) string
	}{
		{
			name: "not wiki-named",
			setup: func(t *testing.T) string {
				parent := t.TempDir()
				root := filepath.Join(parent, "project")
				require.NoError(t, os.MkdirAll(root, 0o755))
				writeSourceRootGitRemote(t, root, "https://github.com/acme/project.git")
				return root
			},
		},
		{
			name: "sibling missing",
			setup: func(t *testing.T) string {
				parent := t.TempDir()
				wiki := filepath.Join(parent, "project.wiki")
				require.NoError(t, os.MkdirAll(wiki, 0o755))
				writeSourceRootGitRemote(t, wiki, "https://github.com/acme/project.wiki.git")
				return wiki
			},
		},
		{
			name: "sibling is a file",
			setup: func(t *testing.T) string {
				parent := t.TempDir()
				wiki := filepath.Join(parent, "project.wiki")
				require.NoError(t, os.MkdirAll(wiki, 0o755))
				writeSourceRootGitRemote(t, wiki, "https://github.com/acme/project.wiki.git")
				require.NoError(t, os.WriteFile(filepath.Join(parent, "project"), []byte("x"), 0o644))
				return wiki
			},
		},
		{
			name: "sibling not a git checkout",
			setup: func(t *testing.T) string {
				parent := t.TempDir()
				wiki := filepath.Join(parent, "project.wiki")
				require.NoError(t, os.MkdirAll(wiki, 0o755))
				writeSourceRootGitRemote(t, wiki, "https://github.com/acme/project.wiki.git")
				require.NoError(t, os.MkdirAll(filepath.Join(parent, "project"), 0o755))
				return wiki
			},
		},
		{
			name: "sibling different repo",
			setup: func(t *testing.T) string {
				parent := t.TempDir()
				wiki := filepath.Join(parent, "project.wiki")
				require.NoError(t, os.MkdirAll(wiki, 0o755))
				writeSourceRootGitRemote(t, wiki, "https://github.com/acme/project.wiki.git")
				src := filepath.Join(parent, "project")
				require.NoError(t, os.MkdirAll(src, 0o755))
				writeSourceRootGitRemote(t, src, "https://github.com/acme/other.git")
				return wiki
			},
		},
		{
			name: "wiki remote absent",
			setup: func(t *testing.T) string {
				parent := t.TempDir()
				wiki := filepath.Join(parent, "project.wiki")
				require.NoError(t, os.MkdirAll(wiki, 0o755))
				src := filepath.Join(parent, "project")
				require.NoError(t, os.MkdirAll(src, 0o755))
				writeSourceRootGitRemote(t, src, "https://github.com/acme/project.git")
				return wiki
			},
		},
		{
			name: "sibling remote absent",
			setup: func(t *testing.T) string {
				parent := t.TempDir()
				wiki := filepath.Join(parent, "project.wiki")
				require.NoError(t, os.MkdirAll(wiki, 0o755))
				writeSourceRootGitRemote(t, wiki, "https://github.com/acme/project.wiki.git")
				src := filepath.Join(parent, "project")
				require.NoError(t, os.MkdirAll(src, 0o755))
				require.NoError(t, os.MkdirAll(filepath.Join(src, ".git"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(src, ".git", "config"), []byte("[core]\n\tbare = false\n"), 0o644))
				return wiki
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			path, origin, err := ResolveSourceRoot(config.Default(), root)
			require.NoError(t, err)
			require.Empty(t, path)
			require.Empty(t, origin)
		})
	}
}

// TestCanonicalRepoIdentity asserts the https, scp, and ssh remote forms all
// reduce to one host/owner/repo identity, that the wiki forms lose their .wiki
// tail to match the source forms, and that sameRepoIdentity requires both sides
// non-empty and equal.
func TestCanonicalRepoIdentity(t *testing.T) {
	const want = "github.com/acme/project"
	wikiForms := []string{
		"https://github.com/acme/project.wiki.git",
		"git@github.com:acme/project.wiki.git",
		"ssh://git@github.com/acme/project.wiki.git",
		"https://github.com/Acme/Project.wiki.git/",
	}
	for _, form := range wikiForms {
		require.Equal(t, want, canonicalRepoIdentity(form), form)
	}
	srcForms := []string{
		"https://github.com/acme/project.git",
		"git@github.com:acme/project.git",
		"ssh://git@github.com/acme/project.git",
	}
	for _, form := range srcForms {
		require.Equal(t, want, canonicalRepoIdentity(form), form)
	}

	require.True(t, sameRepoIdentity("https://github.com/acme/project.wiki.git", "git@github.com:acme/project.git"))
	require.False(t, sameRepoIdentity("https://github.com/acme/project.wiki.git", "https://github.com/acme/other.git"))
	require.False(t, sameRepoIdentity("", "https://github.com/acme/project.git"))
	require.False(t, sameRepoIdentity("https://github.com/acme/project.wiki.git", ""))
	require.Empty(t, canonicalRepoIdentity(""))
}

// TestStripWikiSuffix pins the basename-to-sibling-name reduction used to derive
// the single probe candidate.
func TestStripWikiSuffix(t *testing.T) {
	require.Equal(t, "project", stripWikiSuffix("project.wiki"))
	require.Equal(t, "Project", stripWikiSuffix("Project.wiki"))
	require.Equal(t, "project", stripWikiSuffix("project.wiki.git"))
	require.Equal(t, "", stripWikiSuffix("project"))
	require.Equal(t, "", stripWikiSuffix(".wiki"))
}

func writeSourceRootGitRemote(t *testing.T, dir, url string) {
	t.Helper()
	cfgPath := filepath.Join(dir, ".git", "config")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o755))
	content := "[remote \"origin\"]\n\turl = " + url + "\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0o644))
}
