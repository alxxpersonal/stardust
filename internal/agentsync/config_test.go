package agentsync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/alxxpersonal/stardust/internal/config"
)

func TestLayoutSyncConfig(t *testing.T) {
	layout := config.Layout{Root: "/vault"}

	if got, want := layout.SyncConfig(), "/vault/.stardust/sync.toml"; got != want {
		t.Fatalf("SyncConfig() = %q, want %q", got, want)
	}
}

func TestLayoutRules(t *testing.T) {
	layout := config.Layout{Root: "/vault"}

	if got, want := layout.Rules(), "/vault/.stardust/rules.md"; got != want {
		t.Fatalf("Rules() = %q, want %q", got, want)
	}
}

func TestDefaultConfigWiresRules(t *testing.T) {
	cfg := DefaultConfig("/home", "/vault", "docs/agents")

	var primary, compat *Source
	for i := range cfg.Sources {
		if cfg.Sources[i].Kind != string(KindRules) {
			continue
		}
		switch cfg.Sources[i].Name {
		case "compat-rules":
			compat = &cfg.Sources[i]
		case "repo-rules":
			primary = &cfg.Sources[i]
		}
	}
	if primary == nil {
		t.Fatal("DefaultConfig() has no primary rules source")
	}
	if got, want := primary.Path, filepath.Join("/vault", "docs", "agents", "rules.md"); got != want {
		t.Fatalf("primary rules source path = %q, want %q", got, want)
	}
	if primary.Priority != 100 {
		t.Fatalf("primary rules source priority = %d, want 100", primary.Priority)
	}
	if compat == nil {
		t.Fatal("DefaultConfig() has no compat rules source")
	}
	if got, want := compat.Path, filepath.Join("/vault", ".stardust", "rules.md"); got != want {
		t.Fatalf("compat rules source path = %q, want %q", got, want)
	}
	// A legacy layout keeps syncing until migrated: the compat source is real,
	// not import-only, and its higher priority number loses collisions to the
	// docs/agents copy.
	if compat.ImportOnly {
		t.Fatal("compat rules source must keep materializing (not import-only)")
	}
	if compat.Priority <= primary.Priority {
		t.Fatalf("compat priority %d must lose to primary %d", compat.Priority, primary.Priority)
	}

	wantRepo := map[Tool]string{
		ToolClaude: filepath.Join("/vault", "CLAUDE.md"),
		ToolCodex:  filepath.Join("/vault", "AGENTS.md"),
		ToolGemini: filepath.Join("/vault", "GEMINI.md"),
	}
	for _, target := range cfg.Targets {
		if target.Scope == ScopeRepo {
			if got, want := target.RulesPath, wantRepo[target.Tool]; got != want {
				t.Fatalf("repo %s RulesPath = %q, want %q", target.Tool, got, want)
			}
		}
		if target.Scope == ScopeGlobal && target.RulesPath != "" {
			t.Fatalf("global %s RulesPath = %q, want empty", target.Tool, target.RulesPath)
		}
	}
}

// TestDefaultConfigHomesAgentAssetsInDocs asserts the repo sources point at the
// docs/agents home at Priority 100, and that the legacy repo-root paths stay
// REAL compat sources (still materialized, so a legacy layout keeps syncing) at
// a lower precedence (a higher priority number), so docs/agents wins any
// collision.
func TestDefaultConfigHomesAgentAssetsInDocs(t *testing.T) {
	cfg := DefaultConfig("/home", "/vault", "docs/agents")
	byName := map[string]Source{}
	for _, s := range cfg.Sources {
		byName[s.Name] = s
	}

	type want struct {
		path       string
		kind       string
		priority   int
		importOnly bool
	}
	cases := map[string]want{
		"repo-skills":   {filepath.Join("/vault", "docs", "agents", "skills"), "skill", 100, false},
		"repo-agents":   {filepath.Join("/vault", "docs", "agents", "subagents"), "agent", 100, false},
		"repo-rules":    {filepath.Join("/vault", "docs", "agents", "rules.md"), "rules", 100, false},
		"compat-skills": {filepath.Join("/vault", "skills"), "skill", 200, false},
		"compat-agents": {filepath.Join("/vault", "agents"), "agent", 200, false},
		"compat-rules":  {filepath.Join("/vault", ".stardust", "rules.md"), "rules", 200, false},
	}
	for name, w := range cases {
		s, ok := byName[name]
		if !ok {
			t.Fatalf("DefaultConfig() missing source %q", name)
		}
		if s.Path != w.path || s.Kind != w.kind || s.Priority != w.priority || s.ImportOnly != w.importOnly {
			t.Fatalf("source %q = %#v, want path=%q kind=%q priority=%d importOnly=%v", name, s, w.path, w.kind, w.priority, w.importOnly)
		}
	}

	// A higher priority number loses in Discover, so every compat source is lower
	// precedence than its docs/agents counterpart: the docs/agents copy wins.
	for _, kind := range []string{"skills", "agents", "rules"} {
		if byName["compat-"+kind].Priority <= byName["repo-"+kind].Priority {
			t.Fatalf("compat-%s priority %d must be a higher number than repo-%s priority %d", kind, byName["compat-"+kind].Priority, kind, byName["repo-"+kind].Priority)
		}
	}
}

// TestDiscoverPrefersDocsAgentsOverCompat pins the collision rule: an item with
// the same name in both the docs/agents home and the legacy root skills dir
// resolves to the docs/agents copy, and that winner is not import-only (it
// materializes into tool dirs).
func TestDiscoverPrefersDocsAgentsOverCompat(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeSkill(t, filepath.Join(root, "docs", "agents", "skills"), "shared",
		"---\nname: shared\ndescription: docs-homed copy\n---\n# Shared (docs)\n")
	writeSkill(t, filepath.Join(root, "skills"), "shared",
		"---\nname: shared\ndescription: legacy copy\n---\n# Shared (legacy)\n")

	items, err := Discover(DefaultConfig(home, root, "docs/agents"))
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	var got []Item
	for _, it := range items {
		if it.Kind == KindSkill && it.Name == "shared" {
			got = append(got, it)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want exactly one resolved 'shared' skill, got %d: %#v", len(got), got)
	}
	wantSource := filepath.Join(root, "docs", "agents", "skills", "shared")
	if got[0].SourcePath != wantSource {
		t.Fatalf("collision winner source = %q, want docs/agents copy %q", got[0].SourcePath, wantSource)
	}
	if got[0].Source.ImportOnly {
		t.Fatal("docs/agents winner must not be import-only")
	}
}

// TestDefaultConfigHomesReposUnderConfiguredDir asserts the three repo sources
// build under the passed agents dir instead of a hardcoded docs/agents, while the
// legacy compat sources stay at the repo root regardless of the knob.
func TestDefaultConfigHomesReposUnderConfiguredDir(t *testing.T) {
	cfg := DefaultConfig("/home", "/vault", "docs/skills")
	byName := map[string]Source{}
	for _, s := range cfg.Sources {
		byName[s.Name] = s
	}
	wantRepo := map[string]string{
		"repo-skills": filepath.Join("/vault", "docs", "skills", "skills"),
		"repo-agents": filepath.Join("/vault", "docs", "skills", "subagents"),
		"repo-rules":  filepath.Join("/vault", "docs", "skills", "rules.md"),
	}
	for name, want := range wantRepo {
		if got := byName[name].Path; got != want {
			t.Fatalf("source %q path = %q, want %q", name, got, want)
		}
	}
	if got, want := byName["compat-skills"].Path, filepath.Join("/vault", "skills"); got != want {
		t.Fatalf("compat-skills path = %q, want %q (unaffected by knob)", got, want)
	}
	if got, want := byName["compat-rules"].Path, filepath.Join("/vault", ".stardust", "rules.md"); got != want {
		t.Fatalf("compat-rules path = %q, want %q (unaffected by knob)", got, want)
	}
}

func TestLoadConfigExpandsHomeAndRepoRelativePaths(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	root := filepath.Join(dir, "vault")
	path := filepath.Join(dir, "sync.toml")
	body := []byte(`
default_targets = ["claude"]

[[sources]]
name = "canonical"
path = "~/skills"
kind = "skill"
priority = 10

[[targets]]
tool = "claude"
scope = "repo"
skills_path = ".claude/skills"
agents_path = ".claude/agents"
mode = "copy"
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path, home, root, "docs/agents")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if got, want := cfg.Sources[0].Path, filepath.Join(home, "skills"); got != want {
		t.Fatalf("source path = %q, want %q", got, want)
	}
	if got, want := cfg.Targets[0].SkillsPath, filepath.Join(root, ".claude/skills"); got != want {
		t.Fatalf("target skills path = %q, want %q", got, want)
	}
	if got, want := cfg.Targets[0].AgentsPath, filepath.Join(root, ".claude/agents"); got != want {
		t.Fatalf("target agents path = %q, want %q", got, want)
	}
	if got, want := cfg.DefaultTargets, []Tool{ToolClaude}; !reflect.DeepEqual(got, want) {
		t.Fatalf("default targets = %#v, want %#v", got, want)
	}
}

func TestLoadConfigMissingReturnsDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	root := filepath.Join(dir, "vault")

	cfg, err := LoadConfig(filepath.Join(dir, "missing.toml"), home, root, "docs/agents")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	want := DefaultConfig(home, root, "docs/agents")
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("LoadConfig() = %#v, want %#v", cfg, want)
	}
}
