package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alxxpersonal/stardust/internal/agentsync"
	"github.com/alxxpersonal/stardust/internal/config"
)

func TestSyncDryRunPlansWithoutApplying(t *testing.T) {
	root := t.TempDir()
	requireVaultConfig(t, root)
	source := filepath.Join(root, "skills")
	writeSyncSkill(t, source, "foo")
	writeSyncConfig(t, root, source, filepath.Join(root, ".claude", "skills"))

	svc, err := Open(t.Context(), root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = svc.Close() }()

	res, err := svc.Sync(t.Context(), agentsync.Options{Scope: agentsync.ScopeRepo, Tools: []agentsync.Tool{agentsync.ToolClaude}, DryRun: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if got, want := len(res.Plan.Actions), 1; got != want {
		t.Fatalf("len(Actions) = %d, want %d", got, want)
	}
	if got, want := res.Plan.Actions[0].Status, "create"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "skills", "foo")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created target or unexpected stat error: %v", err)
	}
}

func TestSyncComposesRulesIntoRootFiles(t *testing.T) {
	root := t.TempDir()
	requireVaultConfig(t, root)
	layout := config.Layout{Root: root}

	if err := os.WriteFile(layout.Rules(), []byte("---\nname: rules\ntargets: [claude, codex, gemini]\n---\n# House rules\n\ncompose, never clobber\n"), 0o644); err != nil {
		t.Fatalf("write rules source: %v", err)
	}
	// A pre-existing CLAUDE.md with human content the sync must preserve.
	claudePath := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte("# My project\n\nhand-written intro\n"), 0o644); err != nil {
		t.Fatalf("write user CLAUDE.md: %v", err)
	}
	writeRulesSyncConfig(t, root)

	svc, err := Open(t.Context(), root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = svc.Close() }()

	opts := agentsync.Options{Scope: agentsync.ScopeRepo}
	if _, err := svc.Sync(t.Context(), opts); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		text := string(body)
		if !strings.Contains(text, "compose, never clobber") {
			t.Fatalf("%s missing composed rules: %q", name, text)
		}
		if !strings.Contains(text, "stardust rules") {
			t.Fatalf("%s missing sentinel marker: %q", name, text)
		}
		if strings.Contains(text, "name: rules") {
			t.Fatalf("%s leaked frontmatter: %q", name, text)
		}
	}
	claude, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(claude), "hand-written intro") {
		t.Fatalf("CLAUDE.md dropped user content: %q", string(claude))
	}

	// In sync: check reports nothing.
	checkOpts := agentsync.Options{Scope: agentsync.ScopeRepo, Check: true}
	res, err := svc.Sync(t.Context(), checkOpts)
	if err != nil {
		t.Fatalf("Sync(check) error = %v", err)
	}
	if res.Plan.Missing != 0 || res.Plan.Drift != 0 || res.Plan.Conflicts != 0 {
		t.Fatalf("in-sync check: missing=%d drift=%d conflicts=%d, want all 0", res.Plan.Missing, res.Plan.Drift, res.Plan.Conflicts)
	}

	// Edit the canonical source: check must now report drift.
	if err := os.WriteFile(layout.Rules(), []byte("---\nname: rules\ntargets: [claude, codex, gemini]\n---\n# House rules\n\ncompose, never clobber, v2\n"), 0o644); err != nil {
		t.Fatalf("edit rules source: %v", err)
	}
	res, err = svc.Sync(t.Context(), checkOpts)
	if err != nil {
		t.Fatalf("Sync(check after edit) error = %v", err)
	}
	if res.Plan.Drift != 3 {
		t.Fatalf("post-edit check drift = %d, want 3", res.Plan.Drift)
	}

	// A plain sync heals the drift without --repair.
	if _, err := svc.Sync(t.Context(), opts); err != nil {
		t.Fatalf("Sync(heal) error = %v", err)
	}
	res, err = svc.Sync(t.Context(), checkOpts)
	if err != nil {
		t.Fatalf("Sync(check after heal) error = %v", err)
	}
	if res.Plan.Missing != 0 || res.Plan.Drift != 0 || res.Plan.Conflicts != 0 {
		t.Fatalf("post-heal check: missing=%d drift=%d conflicts=%d, want all 0", res.Plan.Missing, res.Plan.Drift, res.Plan.Conflicts)
	}
	healed, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read healed CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(healed), "compose, never clobber, v2") {
		t.Fatalf("healed CLAUDE.md missing v2 body: %q", string(healed))
	}
	if !strings.Contains(string(healed), "hand-written intro") {
		t.Fatalf("healed CLAUDE.md dropped user content: %q", string(healed))
	}
}

func writeRulesSyncConfig(t *testing.T, root string) {
	t.Helper()
	layout := config.Layout{Root: root}
	body := "default_targets = [\"claude\", \"codex\", \"gemini\"]\n\n" +
		"[[sources]]\n" +
		"name = \"repo-rules\"\n" +
		"path = \"" + filepath.ToSlash(layout.Rules()) + "\"\n" +
		"kind = \"rules\"\n" +
		"priority = 100\n\n" +
		rulesTargetBlock("claude", filepath.Join(root, "CLAUDE.md")) +
		rulesTargetBlock("codex", filepath.Join(root, "AGENTS.md")) +
		rulesTargetBlock("gemini", filepath.Join(root, "GEMINI.md"))
	if err := os.MkdirAll(layout.Dir(), 0o755); err != nil {
		t.Fatalf("create stardust dir: %v", err)
	}
	if err := os.WriteFile(layout.SyncConfig(), []byte(body), 0o644); err != nil {
		t.Fatalf("write sync config: %v", err)
	}
}

func rulesTargetBlock(tool, rulesPath string) string {
	return "[[targets]]\n" +
		"tool = \"" + tool + "\"\n" +
		"scope = \"repo\"\n" +
		"rules_path = \"" + filepath.ToSlash(rulesPath) + "\"\n\n"
}

func requireVaultConfig(t *testing.T, root string) {
	t.Helper()
	layout := config.Layout{Root: root}
	if err := os.MkdirAll(layout.Cache(), 0o755); err != nil {
		t.Fatalf("create cache: %v", err)
	}
	if err := config.Save(layout.Config(), config.Default()); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func writeSyncSkill(t *testing.T, source, name string) {
	t.Helper()
	dir := filepath.Join(source, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func writeSyncConfig(t *testing.T, root, source, target string) {
	t.Helper()
	layout := config.Layout{Root: root}
	body := "default_targets = [\"claude\"]\n\n" +
		"[[sources]]\n" +
		"name = \"skills\"\n" +
		"path = \"" + filepath.ToSlash(source) + "\"\n" +
		"kind = \"skill\"\n" +
		"priority = 10\n\n" +
		"[[targets]]\n" +
		"tool = \"claude\"\n" +
		"scope = \"repo\"\n" +
		"skills_path = \"" + filepath.ToSlash(target) + "\"\n" +
		"mode = \"symlink\"\n"
	if err := os.MkdirAll(layout.Dir(), 0o755); err != nil {
		t.Fatalf("create stardust dir: %v", err)
	}
	if err := os.WriteFile(layout.SyncConfig(), []byte(body), 0o644); err != nil {
		t.Fatalf("write sync config: %v", err)
	}
}
