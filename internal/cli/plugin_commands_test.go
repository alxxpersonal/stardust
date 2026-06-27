package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPluginAuthoringCommandsAreInline(t *testing.T) {
	root := repoRootFromCaller(t)
	commands := []string{"spec.md", "plan.md", "doc.md", "adr.md"}

	for _, name := range commands {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "plugin", "claude", "commands", name))
			require.NoError(t, err)
			body := string(data)

			require.Contains(t, body, "allowed-tools: Bash, Read, Write")
			require.Contains(t, body, "resolve-root.sh")
			require.Contains(t, body, "stardust registry")
			require.Contains(t, body, "Do not print a second slash command")
		})
	}
}

func TestPluginMetadataDescribesInlineAuthoring(t *testing.T) {
	root := repoRootFromCaller(t)

	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		require.NoError(t, err)
		return string(data)
	}

	readme := read("plugin/claude/README.md")
	require.Contains(t, readme, "author docs directly")
	require.NotContains(t, readme, "hands off")
	require.NotContains(t, readme, "handoff")

	pluginJSON := read("plugin/claude/.claude-plugin/plugin.json")
	require.Contains(t, pluginJSON, "run the docs write workflows inline")
	require.NotContains(t, pluginJSON, "delegate to")
}

func repoRootFromCaller(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if strings.HasSuffix(root, string(filepath.Separator)+"internal") {
		root = filepath.Dir(root)
	}
	return root
}
