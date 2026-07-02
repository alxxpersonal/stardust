package agentsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPlanClassifiesTargetState(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "sources", "skills", "foo")
	other := filepath.Join(root, "sources", "skills", "other")
	targets := filepath.Join(root, "targets")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatalf("create other source: %v", err)
	}

	cfg := Config{Targets: []Target{
		{Tool: ToolClaude, Scope: ScopeRepo, SkillsPath: filepath.Join(targets, "missing"), Mode: "symlink"},
		{Tool: ToolCodex, Scope: ScopeRepo, SkillsPath: filepath.Join(targets, "ok"), Mode: "symlink"},
		{Tool: ToolGemini, Scope: ScopeRepo, SkillsPath: filepath.Join(targets, "drift"), Mode: "symlink"},
		{Tool: ToolClaude, Scope: ScopeGlobal, SkillsPath: filepath.Join(targets, "conflict"), Mode: "symlink"},
	}}
	item := Item{Name: "foo", Kind: KindSkill, SourcePath: source, Targets: []Tool{ToolClaude, ToolCodex, ToolGemini}}

	mustSymlink(t, source, filepath.Join(targets, "ok", "foo"))
	mustSymlink(t, other, filepath.Join(targets, "drift", "foo"))
	if err := os.MkdirAll(filepath.Join(targets, "conflict", "foo"), 0o755); err != nil {
		t.Fatalf("create conflict target: %v", err)
	}

	plan, err := BuildPlan(cfg, []Item{item}, Options{Scope: ScopeAll})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}

	got := map[string]Action{}
	for _, action := range plan.Actions {
		got[string(action.Tool)+"/"+string(action.Scope)] = action
	}
	assertActionStatus(t, got["claude/repo"], "create", filepath.Join(targets, "missing", "foo"))
	assertActionStatus(t, got["codex/repo"], "ok", filepath.Join(targets, "ok", "foo"))
	assertActionStatus(t, got["gemini/repo"], "drift", filepath.Join(targets, "drift", "foo"))
	assertActionStatus(t, got["claude/global"], "conflict", filepath.Join(targets, "conflict", "foo"))
	if got, want := plan.Missing, 1; got != want {
		t.Fatalf("Missing = %d, want %d", got, want)
	}
	if got, want := plan.Drift, 1; got != want {
		t.Fatalf("Drift = %d, want %d", got, want)
	}
	if got, want := plan.Conflicts, 1; got != want {
		t.Fatalf("Conflicts = %d, want %d", got, want)
	}
}

func TestBuildPlanClassifiesRulesState(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".stardust", "rules.md")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(source, []byte("---\nname: rules\n---\n# Rules\n\nbe kind\n"), 0o644); err != nil {
		t.Fatalf("write rules source: %v", err)
	}
	rendered := renderRulesBody("---\nname: rules\n---\n# Rules\n\nbe kind\n")

	claudePath := filepath.Join(root, "CLAUDE.md") // create (missing)
	codexPath := filepath.Join(root, "AGENTS.md")  // ok (matching block)
	geminiPath := filepath.Join(root, "GEMINI.md") // drift (stale block)

	if err := os.WriteFile(codexPath, []byte("# Repo owned\n"), 0o644); err != nil {
		t.Fatalf("write codex target: %v", err)
	}
	if err := injectRulesBlock(codexPath, rendered); err != nil {
		t.Fatalf("seed ok block: %v", err)
	}
	if err := injectRulesBlock(geminiPath, "stale body"); err != nil {
		t.Fatalf("seed drift block: %v", err)
	}

	cfg := Config{Targets: []Target{
		{Tool: ToolClaude, Scope: ScopeRepo, RulesPath: claudePath, Mode: "symlink"},
		{Tool: ToolCodex, Scope: ScopeRepo, RulesPath: codexPath, Mode: "symlink"},
		{Tool: ToolGemini, Scope: ScopeRepo, RulesPath: geminiPath, Mode: "symlink"},
		{Tool: ToolClaude, Scope: ScopeGlobal, RulesPath: "", Mode: "symlink"}, // skipped, empty path
	}}
	item := Item{Name: "rules", Kind: KindRules, SourcePath: source, Targets: []Tool{ToolClaude, ToolCodex, ToolGemini}}

	plan, err := BuildPlan(cfg, []Item{item}, Options{Scope: ScopeAll})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}

	got := map[string]Action{}
	for _, action := range plan.Actions {
		got[string(action.Tool)+"/"+string(action.Scope)] = action
	}
	if len(plan.Actions) != 3 {
		t.Fatalf("len(Actions) = %d, want 3 (empty-path target skipped): %#v", len(plan.Actions), plan.Actions)
	}
	assertActionStatus(t, got["claude/repo"], "create", claudePath)
	assertActionStatus(t, got["codex/repo"], "ok", codexPath)
	assertActionStatus(t, got["gemini/repo"], "drift", geminiPath)
	if _, ok := got["claude/global"]; ok {
		t.Fatal("empty RulesPath target produced an action, want it skipped")
	}
	for _, action := range plan.Actions {
		if action.Mode != "compose" {
			t.Fatalf("rules action mode = %q, want compose", action.Mode)
		}
		if action.Status == "conflict" {
			t.Fatal("rules produced a conflict, want never")
		}
	}
	if got, want := plan.Missing, 1; got != want {
		t.Fatalf("Missing = %d, want %d", got, want)
	}
	if got, want := plan.Drift, 1; got != want {
		t.Fatalf("Drift = %d, want %d", got, want)
	}
	if got, want := plan.Conflicts, 0; got != want {
		t.Fatalf("Conflicts = %d, want %d", got, want)
	}
}

func TestBuildPlanFiltersTools(t *testing.T) {
	root := t.TempDir()
	item := Item{
		Name:       "foo",
		Kind:       KindSkill,
		SourcePath: filepath.Join(root, "foo"),
		Targets:    []Tool{ToolClaude, ToolCodex, ToolGemini},
	}
	cfg := Config{Targets: []Target{
		{Tool: ToolClaude, Scope: ScopeRepo, SkillsPath: filepath.Join(root, "claude"), Mode: "symlink"},
		{Tool: ToolCodex, Scope: ScopeRepo, SkillsPath: filepath.Join(root, "codex"), Mode: "symlink"},
		{Tool: ToolGemini, Scope: ScopeRepo, SkillsPath: filepath.Join(root, "gemini"), Mode: "symlink"},
	}}

	plan, err := BuildPlan(cfg, []Item{item}, Options{Scope: ScopeAll, Tools: []Tool{ToolClaude}})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if got, want := len(plan.Actions), 1; got != want {
		t.Fatalf("len(Actions) = %d, want %d", got, want)
	}
	if got, want := plan.Actions[0].Tool, ToolClaude; got != want {
		t.Fatalf("action tool = %q, want %q", got, want)
	}
}

func TestBuildPlanSortsActionsDeterministically(t *testing.T) {
	root := t.TempDir()
	cfg := Config{Targets: []Target{
		{Tool: ToolGemini, Scope: ScopeRepo, SkillsPath: filepath.Join(root, "gemini", "skills"), AgentsPath: filepath.Join(root, "gemini", "agents"), Mode: "symlink"},
		{Tool: ToolClaude, Scope: ScopeGlobal, SkillsPath: filepath.Join(root, "claude-global", "skills"), AgentsPath: filepath.Join(root, "claude-global", "agents"), Mode: "symlink"},
		{Tool: ToolClaude, Scope: ScopeRepo, SkillsPath: filepath.Join(root, "claude", "skills"), AgentsPath: filepath.Join(root, "claude", "agents"), Mode: "symlink"},
	}}
	items := []Item{
		{Name: "zeta", Kind: KindSkill, SourcePath: filepath.Join(root, "zeta"), Targets: []Tool{ToolClaude, ToolGemini}},
		{Name: "alpha", Kind: KindAgent, SourcePath: filepath.Join(root, "alpha.md"), Targets: []Tool{ToolClaude}},
	}

	plan, err := BuildPlan(cfg, items, Options{Scope: ScopeAll})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}

	var got []string
	for _, action := range plan.Actions {
		got = append(got, string(action.Tool)+"/"+string(action.Scope)+"/"+string(action.Kind)+"/"+action.ItemName)
	}
	want := []string{
		"claude/global/agent/alpha",
		"claude/global/skill/zeta",
		"claude/repo/agent/alpha",
		"claude/repo/skill/zeta",
		"gemini/repo/skill/zeta",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("action order:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestPlanMarkdownContainsCountsAndRows(t *testing.T) {
	plan := Plan{
		Missing:   1,
		Drift:     1,
		Conflicts: 1,
		Actions: []Action{
			{Kind: KindSkill, ItemName: "foo", Tool: ToolClaude, Scope: ScopeRepo, Source: "/src/foo", Target: "/dst/foo", Mode: "symlink", Status: "create", Reason: "missing"},
		},
	}

	got := plan.Markdown()
	for _, want := range []string{"missing: 1", "drift: 1", "conflicts: 1", "| skill | foo | claude | repo | create |"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Markdown() missing %q:\n%s", want, got)
		}
	}
}

func mustSymlink(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(newname), 0o755); err != nil {
		t.Fatalf("create symlink parent: %v", err)
	}
	if err := os.Symlink(oldname, newname); err != nil {
		t.Fatalf("symlink %s to %s: %v", newname, oldname, err)
	}
}

func assertActionStatus(t *testing.T, action Action, status, target string) {
	t.Helper()
	if got := action.Status; got != status {
		t.Fatalf("status = %q, want %q for %#v", got, status, action)
	}
	if got := action.Target; got != target {
		t.Fatalf("target = %q, want %q", got, target)
	}
}
