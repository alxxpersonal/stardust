package convention

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDetectKind table-tests the directory classifier across the documented
// precedence: obsidian wins outright, then a source marker, then markdown
// dominance, then a bare .git, otherwise a plain vault.
func TestDetectKind(t *testing.T) {
	type file struct {
		name string
		dir  bool
	}
	cases := []struct {
		name  string
		files []file
		want  Kind
	}{
		{
			name:  "go.mod and .git is a code repo",
			files: []file{{name: "go.mod"}, {name: ".git", dir: true}},
			want:  KindCodeRepo,
		},
		{
			name:  "only markdown is a plain vault",
			files: []file{{name: "a.md"}, {name: "b.md"}},
			want:  KindPlainVault,
		},
		{
			name:  "obsidian dir is a plain vault",
			files: []file{{name: ".obsidian", dir: true}, {name: "a.md"}},
			want:  KindPlainVault,
		},
		{
			name:  "empty dir is a plain vault",
			files: nil,
			want:  KindPlainVault,
		},
		{
			name:  "bare git with a stray non-markdown file is a code repo",
			files: []file{{name: ".git", dir: true}, {name: "notes.txt"}},
			want:  KindCodeRepo,
		},
		{
			name:  "source marker beats markdown count",
			files: []file{{name: "main.go"}, {name: "a.md"}, {name: "b.md"}},
			want:  KindCodeRepo,
		},
		{
			name:  "obsidian wins over a source marker",
			files: []file{{name: ".obsidian", dir: true}, {name: "main.go"}},
			want:  KindPlainVault,
		},
		{
			name:  "flat github wiki is a github wiki",
			files: []file{{name: "Home.md"}, {name: "_Sidebar.md"}, {name: "Install-Guide.md"}},
			want:  KindGitHubWiki,
		},
		{
			name:  "docs directory prevents flat wiki heuristic",
			files: []file{{name: "docs", dir: true}, {name: "Home.md"}, {name: "_Sidebar.md"}, {name: "Install-Guide.md"}},
			want:  KindPlainVault,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tc.files {
				p := filepath.Join(dir, f.name)
				if f.dir {
					require.NoError(t, os.MkdirAll(p, 0o755))
					continue
				}
				require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
			}
			got, err := DetectKind(dir)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDetectKindGitHubWikiSignals(t *testing.T) {
	parent := t.TempDir()
	dirNamedWiki := filepath.Join(parent, "project.wiki")
	require.NoError(t, os.MkdirAll(dirNamedWiki, 0o755))
	writeDetectFile(t, dirNamedWiki, "go.mod", "module example.com/wiki\n")

	got, err := DetectKind(dirNamedWiki)
	require.NoError(t, err)
	require.Equal(t, KindGitHubWiki, got)

	remoteWiki := t.TempDir()
	writeDetectFile(t, remoteWiki, ".git/config", "[remote \"origin\"]\n\turl = https://github.com/acme/project.wiki.git\n")
	writeDetectFile(t, remoteWiki, "go.mod", "module example.com/wiki\n")

	got, err = DetectKind(remoteWiki)
	require.NoError(t, err)
	require.Equal(t, KindGitHubWiki, got)
}

// TestKindMethods pins the stable labels, the docs default, and the override
// flag named in each describe sentence.
func TestKindMethods(t *testing.T) {
	require.True(t, KindCodeRepo.WantsDocs())
	require.False(t, KindPlainVault.WantsDocs())
	require.False(t, KindGitHubWiki.WantsDocs())

	require.Equal(t, "code-repo-with-docs", KindCodeRepo.Label())
	require.Equal(t, "plain-vault", KindPlainVault.Label())
	require.Equal(t, "github-wiki", KindGitHubWiki.Label())

	require.Contains(t, KindCodeRepo.Describe(), "--no-docs")
	require.Contains(t, KindPlainVault.Describe(), "--docs")
	require.Contains(t, KindGitHubWiki.Describe(), "--docs")
}

func writeDetectFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
