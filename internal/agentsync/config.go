// Package agentsync discovers, plans, and applies Stardust-managed agent
// assets across supported AI tool directories.
package agentsync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Tool identifies an AI tool target.
type Tool string

// Supported sync tools.
const (
	ToolClaude Tool = "claude"
	ToolCodex  Tool = "codex"
	ToolGemini Tool = "gemini"
)

// Scope identifies whether a target is repo-local or global.
type Scope string

// Supported sync scopes.
const (
	ScopeRepo   Scope = "repo"
	ScopeGlobal Scope = "global"
	ScopeAll    Scope = "all"
)

// Source declares an input folder for skills or agents.
type Source struct {
	Name       string `toml:"name" json:"name"`
	Path       string `toml:"path" json:"path"`
	Kind       string `toml:"kind" json:"kind"`
	Priority   int    `toml:"priority" json:"priority"`
	ImportOnly bool   `toml:"import_only" json:"import_only"`
}

// Target declares a tool directory that Stardust can maintain.
type Target struct {
	Tool       Tool   `toml:"tool" json:"tool"`
	Scope      Scope  `toml:"scope" json:"scope"`
	SkillsPath string `toml:"skills_path" json:"skills_path"`
	AgentsPath string `toml:"agents_path" json:"agents_path"`
	Mode       string `toml:"mode" json:"mode"`
}

// Config is the sync.toml schema.
type Config struct {
	Sources        []Source `toml:"sources" json:"sources"`
	Targets        []Target `toml:"targets" json:"targets"`
	DefaultTargets []Tool   `toml:"default_targets" json:"default_targets"`
}

// DefaultConfig returns a generic repo-first sync configuration.
func DefaultConfig(home, root string) Config {
	return Config{
		Sources: []Source{
			{Name: "repo-skills", Path: filepath.Join(root, "skills"), Kind: "skill", Priority: 100},
			{Name: "repo-agents", Path: filepath.Join(root, "agents"), Kind: "agent", Priority: 100},
		},
		Targets: []Target{
			{Tool: ToolClaude, Scope: ScopeRepo, SkillsPath: filepath.Join(root, ".claude", "skills"), AgentsPath: filepath.Join(root, ".claude", "agents"), Mode: "symlink"},
			{Tool: ToolCodex, Scope: ScopeRepo, SkillsPath: filepath.Join(root, ".codex", "skills"), AgentsPath: filepath.Join(root, ".codex", "agents"), Mode: "symlink"},
			{Tool: ToolGemini, Scope: ScopeRepo, SkillsPath: filepath.Join(root, ".gemini", "skills"), AgentsPath: filepath.Join(root, ".gemini", "agents"), Mode: "symlink"},
			{Tool: ToolClaude, Scope: ScopeGlobal, SkillsPath: filepath.Join(home, ".claude", "skills"), AgentsPath: filepath.Join(home, ".claude", "agents"), Mode: "symlink"},
			{Tool: ToolCodex, Scope: ScopeGlobal, SkillsPath: filepath.Join(home, ".codex", "skills"), AgentsPath: filepath.Join(home, ".codex", "agents"), Mode: "symlink"},
			{Tool: ToolGemini, Scope: ScopeGlobal, SkillsPath: filepath.Join(home, ".gemini", "skills"), AgentsPath: filepath.Join(home, ".gemini", "agents"), Mode: "symlink"},
		},
		DefaultTargets: []Tool{ToolClaude, ToolCodex, ToolGemini},
	}
}

// AlxxMigrationConfig returns the opinionated migration layout for alxx's
// canonical forge-private agent infrastructure source.
func AlxxMigrationConfig(home, root string) Config {
	canonical := filepath.Join(home, "Code", "Self", "forge-private")
	cfg := DefaultConfig(home, root)
	cfg.Sources = []Source{
		{Name: "canonical-skills", Path: filepath.Join(canonical, "skills"), Kind: "skill", Priority: 0},
		{Name: "canonical-agents", Path: filepath.Join(canonical, "agents"), Kind: "agent", Priority: 0},
		{Name: "forge-skills", Path: filepath.Join(home, "Code", "Self", "forge", "skills"), Kind: "skill", Priority: 20, ImportOnly: true},
		{Name: "shared-agent-skills", Path: filepath.Join(home, ".agents", "skills"), Kind: "skill", Priority: 30, ImportOnly: true},
		{Name: "claude-global-skills", Path: filepath.Join(home, ".claude", "skills"), Kind: "skill", Priority: 40, ImportOnly: true},
		{Name: "claude-global-agents", Path: filepath.Join(home, ".claude", "agents"), Kind: "agent", Priority: 40, ImportOnly: true},
	}
	return cfg
}

// LoadConfig reads sync.toml, expands paths, and validates tool and scope names.
func LoadConfig(path, home, root string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(home, root), nil
		}
		return Config{}, fmt.Errorf("read sync config %s: %w", path, err)
	}
	cfg := DefaultConfig(home, root)
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse sync config %s: %w", path, err)
	}
	if len(cfg.DefaultTargets) == 0 {
		cfg.DefaultTargets = []Tool{ToolClaude, ToolCodex, ToolGemini}
	}
	if err := normalizeConfig(&cfg, home, root); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// MarshalConfig encodes cfg as sync.toml.
func MarshalConfig(cfg Config) ([]byte, error) {
	b, err := toml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal sync config: %w", err)
	}
	return b, nil
}

// SaveConfig writes cfg to path as TOML.
func SaveConfig(path string, cfg Config) error {
	b, err := MarshalConfig(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create sync config dir: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write sync config %s: %w", path, err)
	}
	return nil
}

func normalizeConfig(cfg *Config, home, root string) error {
	for i := range cfg.DefaultTargets {
		if err := validateTool(cfg.DefaultTargets[i]); err != nil {
			return fmt.Errorf("default target %d: %w", i, err)
		}
	}
	for i := range cfg.Sources {
		s := &cfg.Sources[i]
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("source %d: name is required", i)
		}
		switch s.Kind {
		case "skill", "agent":
		default:
			return fmt.Errorf("source %s: unsupported kind %q", s.Name, s.Kind)
		}
		s.Path = expandPath(s.Path, home, root)
	}
	for i := range cfg.Targets {
		t := &cfg.Targets[i]
		if err := validateTool(t.Tool); err != nil {
			return fmt.Errorf("target %d: %w", i, err)
		}
		if err := validateScope(t.Scope); err != nil {
			return fmt.Errorf("target %d: %w", i, err)
		}
		if t.Mode == "" {
			t.Mode = "symlink"
		}
		switch t.Mode {
		case "symlink", "copy":
		default:
			return fmt.Errorf("target %d: unsupported mode %q", i, t.Mode)
		}
		t.SkillsPath = expandPath(t.SkillsPath, home, root)
		t.AgentsPath = expandPath(t.AgentsPath, home, root)
	}
	return nil
}

func validateTool(tool Tool) error {
	switch tool {
	case ToolClaude, ToolCodex, ToolGemini:
		return nil
	default:
		return fmt.Errorf("unsupported tool %q", tool)
	}
}

func validateScope(scope Scope) error {
	switch scope {
	case ScopeRepo, ScopeGlobal:
		return nil
	default:
		return fmt.Errorf("unsupported scope %q", scope)
	}
}

func expandPath(path, home, root string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, path)
}
