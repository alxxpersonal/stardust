package agentsync

import (
	"path/filepath"
	"testing"
)

func TestDefaultMigrationConfigIncludesCanonicalAndImportSources(t *testing.T) {
	cfg := DefaultMigrationConfig("/home/user", "/vault")

	if got, want := cfg.Sources[0].Path, "/home/user/skills"; got != want {
		t.Fatalf("canonical skills path = %q, want %q", got, want)
	}
	if got, want := cfg.Sources[1].Path, "/home/user/agents"; got != want {
		t.Fatalf("canonical agents path = %q, want %q", got, want)
	}

	wantImport := map[string]bool{
		"/home/user/.agents/skills": true,
		"/home/user/.claude/skills": true,
	}
	for _, src := range cfg.Sources {
		if wantImport[src.Path] && !src.ImportOnly {
			t.Fatalf("source %s should be import-only", src.Path)
		}
		delete(wantImport, src.Path)
	}
	if len(wantImport) > 0 {
		t.Fatalf("missing import-only sources: %#v", wantImport)
	}
}

func TestMigrationReportClassifiesLooseClaudeSkillsAsAdoptable(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, "canonical", "skills")
	loose := filepath.Join(root, ".claude", "skills")
	writeSkill(t, loose, "loose", "---\nname: loose\n---\n# Loose\n")

	cfg := Config{Sources: []Source{
		{Name: "canonical-skills", Path: canonical, Kind: "skill", Priority: 0},
		{Name: "claude-global-skills", Path: loose, Kind: "skill", Priority: 40, ImportOnly: true},
	}, DefaultTargets: []Tool{ToolClaude}}
	items, err := Discover(cfg)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	report := BuildMigrationReport(cfg, items)
	if got, want := len(report.Adoptable), 1; got != want {
		t.Fatalf("len(Adoptable) = %d, want %d: %#v", got, want, report)
	}
	if got, want := report.Adoptable[0].Name, "loose"; got != want {
		t.Fatalf("adoptable name = %q, want %q", got, want)
	}
	if got, want := len(report.Loose), 1; got != want {
		t.Fatalf("len(Loose) = %d, want %d", got, want)
	}
}
