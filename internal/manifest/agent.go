package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AgentManifestInput is the compact state needed to render agent boot context.
type AgentManifestInput struct {
	VaultName    string
	RegistryPath string
	IndexPath    string
	ActivePlans  []RegistryRecord
	StaleDocs    []RegistryRecord
}

// WriteAgentManifest writes the dynamic agent boot manifest.
func WriteAgentManifest(path string, input AgentManifestInput) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(renderAgentManifest(input)), 0o644); err != nil {
		return fmt.Errorf("write agent manifest %s: %w", path, err)
	}
	return nil
}

func renderAgentManifest(input AgentManifestInput) string {
	var b strings.Builder
	b.WriteString("# Stardust manifest\n\n")
	if input.VaultName != "" {
		fmt.Fprintf(&b, "Vault: `%s`.\n\n", input.VaultName)
	}
	b.WriteString("## Start here\n\n")
	fmt.Fprintf(&b, "- Search with `stardust query \"<question>\"` before assuming a note is missing.\n")
	fmt.Fprintf(&b, "- Read `%s` for the vault index.\n", input.IndexPath)
	fmt.Fprintf(&b, "- Read `%s` for docs registry status.\n\n", input.RegistryPath)

	b.WriteString("## Active plans\n\n")
	if len(input.ActivePlans) == 0 {
		b.WriteString("- None.\n")
	} else {
		for _, plan := range input.ActivePlans {
			fmt.Fprintf(&b, "- %s `%s`\n", plan.Title, plan.Path)
		}
	}

	b.WriteString("\n## Stale implemented docs\n\n")
	if len(input.StaleDocs) == 0 {
		b.WriteString("- None.\n")
	} else {
		for _, doc := range input.StaleDocs {
			fmt.Fprintf(&b, "- %s `%s`\n", doc.Title, doc.Path)
		}
	}

	b.WriteString("\n## Core conventions\n\n")
	b.WriteString("- Files are source of truth; indexes and registries are derived.\n")
	b.WriteString("- Docs use YAML frontmatter with type, status, related, and governs fields.\n")
	b.WriteString("- Skills and agents may route with targets: claude, codex, gemini.\n")
	b.WriteString("- Run `stardust check --strict` before committing convention docs.\n")
	return b.String()
}
