package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteAgentManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.md")
	input := AgentManifestInput{
		VaultName:    "vault",
		RegistryPath: "docs/INDEX.md",
		IndexPath:    ".stardust/INDEX.md",
		ActivePlans: []RegistryRecord{
			{Title: "Agent Infra Plan", Status: "Active", Path: "docs/plans/2026-06-22-agent-infra.md"},
		},
		StaleDocs: []RegistryRecord{
			{Title: "Implemented Spec", Status: "Implemented", Path: "docs/specs/2026-06-22-implemented.md"},
		},
	}

	require.NoError(t, WriteAgentManifest(path, input))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	got := string(data)

	require.Contains(t, got, "stardust query")
	require.Contains(t, got, ".stardust/INDEX.md")
	require.Contains(t, got, "docs/INDEX.md")
	require.Contains(t, got, "Agent Infra Plan")
	require.Contains(t, got, "Implemented Spec")
	require.LessOrEqual(t, len(strings.Split(strings.TrimRight(got, "\n"), "\n")), 50)
}
