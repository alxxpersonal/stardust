package mounts

import (
	"os"
	"path/filepath"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestParseHitsArray(t *testing.T) {
	m := Mount{Name: "demo"}
	res := &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{
		Text: `{"hits":[{"title":"A","snippet":"alpha","path":"a.md","score":0.9},{"title":"B","snippet":"beta","path":"b.md"}]}`,
	}}}
	hits := m.Parse(res)
	require.Len(t, hits, 2)
	require.Equal(t, "demo", hits[0].Source)
	require.Equal(t, "A", hits[0].Title)
	require.Equal(t, "a.md", hits[0].Ref)
	require.InDelta(t, 0.9, hits[0].Score, 1e-9)
}

func TestParseFallbackText(t *testing.T) {
	m := Mount{Name: "demo"}
	res := &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "just some plain text"}}}
	hits := m.Parse(res)
	require.Len(t, hits, 1)
	require.Contains(t, hits[0].Snippet, "just some plain text")
}

func TestLoadDefaultsTool(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "gmail"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gmail", "config.toml"),
		[]byte("command = \"some-mcp\"\nargs = [\"serve\"]\n"), 0o644))
	ms, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, ms, 1)
	require.Equal(t, "gmail", ms[0].Name)
	require.Equal(t, "query", ms[0].Cfg.Tool) // defaulted
}

func TestLoadMissingDir(t *testing.T) {
	ms, err := Load(filepath.Join(t.TempDir(), "nope"))
	require.NoError(t, err)
	require.Nil(t, ms)
}
