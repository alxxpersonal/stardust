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
	StaleDocs    []StaleDoc
}

// StaleDoc is one stale implemented doc in the boot manifest, carrying enough
// drift detail for an agent to act on it: how many commits the governed code
// moved since the doc, and which files moved.
type StaleDoc struct {
	Title          string
	Path           string
	ChangedCommits int
	Matched        []string
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
			fmt.Fprintf(&b, "- %s stale: %s to %s since doc `%s`\n", doc.Title, commitCount(doc.ChangedCommits), matchedSummary(doc.Matched), doc.Path)
		}
	}

	b.WriteString("\n## Core conventions\n\n")
	b.WriteString("- Files are source of truth; indexes and registries are derived.\n")
	b.WriteString("- Docs use YAML frontmatter with type, status, related, and governs fields.\n")
	b.WriteString("- Skills and agents may route with targets: claude, codex, gemini.\n")
	b.WriteString("- Run `stardust check --strict` before committing convention docs.\n")
	return b.String()
}

// commitCount renders the changed-commit count with a singular or plural noun,
// e.g. "1 commit" or "3 commits".
func commitCount(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}

// matchedSummary renders the moved code files for a stale doc. It caps the
// rendered set at the first three paths and appends a "+N more" suffix so the
// manifest stays within its line budget regardless of how broad the glob is.
func matchedSummary(matched []string) string {
	if len(matched) == 0 {
		return "governed code"
	}
	const cap = 3
	shown := matched
	extra := 0
	if len(matched) > cap {
		shown = matched[:cap]
		extra = len(matched) - cap
	}
	quoted := make([]string, len(shown))
	for i, m := range shown {
		quoted[i] = "`" + m + "`"
	}
	out := strings.Join(quoted, ", ")
	if extra > 0 {
		out = fmt.Sprintf("%s +%d more", out, extra)
	}
	return out
}
