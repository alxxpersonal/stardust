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
	cfg := DefaultConfig("/home", "/vault")

	var src *Source
	for i := range cfg.Sources {
		if cfg.Sources[i].Kind == string(KindRules) {
			src = &cfg.Sources[i]
			break
		}
	}
	if src == nil {
		t.Fatal("DefaultConfig() has no rules source")
	}
	if got, want := src.Path, filepath.Join("/vault", ".stardust", "rules.md"); got != want {
		t.Fatalf("rules source path = %q, want %q", got, want)
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

	cfg, err := LoadConfig(path, home, root)
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

	cfg, err := LoadConfig(filepath.Join(dir, "missing.toml"), home, root)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	want := DefaultConfig(home, root)
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("LoadConfig() = %#v, want %#v", cfg, want)
	}
}
