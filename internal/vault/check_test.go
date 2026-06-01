package vault

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckFileBadFrontmatter(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntitle: [unclosed\n---\n# A\n"), 0o644))
	probs, err := CheckFile(root, "a.md")
	require.NoError(t, err)
	require.Len(t, probs, 1)
	require.Equal(t, "bad-frontmatter", probs[0].Kind)
}

func TestCheckFileMissingTitle(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.md"), []byte("just body text, no title or heading\n"), 0o644))
	probs, err := CheckFile(root, "b.md")
	require.NoError(t, err)
	require.Len(t, probs, 1)
	require.Equal(t, "missing-title", probs[0].Kind)
}

func TestCheckFileClean(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "c.md"), []byte("---\ntitle: C\n---\n# C\nbody\n"), 0o644))
	probs, err := CheckFile(root, "c.md")
	require.NoError(t, err)
	require.Empty(t, probs)
}

func TestCheckFileH1CountsAsTitle(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "d.md"), []byte("# D Title\nbody with only an h1\n"), 0o644))
	probs, err := CheckFile(root, "d.md")
	require.NoError(t, err)
	require.Empty(t, probs)
}
