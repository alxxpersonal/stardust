// Package config loads and locates Stardust's per-vault configuration and the
// standard paths inside a vault's .stardust directory.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// DirName is the per-vault Stardust directory.
const DirName = ".stardust"

// --- Config ---

// Config is the committed per-vault configuration (.stardust/config.toml).
type Config struct {
	EmbedModel    string            `toml:"embed_model"`
	OllamaURL     string            `toml:"ollama_url"`
	Ignore        []string          `toml:"ignore"`
	RerankerURL   string            `toml:"reranker_url"`   // optional cross-encoder endpoint; empty = disabled
	RerankerModel string            `toml:"reranker_model"` // optional model name passed to the reranker
	SourceRoot    string            `toml:"source_root"`    // optional source repo root for wiki or vault docs; empty = same repo only
	Conventions   ConventionsConfig `toml:"conventions"`
}

// ConventionsConfig holds opt-in vault conventions beyond the default markdown
// indexing behavior.
type ConventionsConfig struct {
	DirectoryIndexes DirectoryIndexesConfig `toml:"directory_indexes"`
}

// DirectoryIndexesConfig configures generated per-directory INDEX.md files.
type DirectoryIndexesConfig struct {
	Enabled  bool     `toml:"enabled"`
	Filename string   `toml:"filename"`
	Roots    []string `toml:"roots"`
	Ignore   []string `toml:"ignore"`
	Mode     string   `toml:"mode"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		EmbedModel:    "bge-m3",
		OllamaURL:     "http://localhost:11434",
		Ignore:        []string{".git", ".obsidian", ".stardust", ".trash", "node_modules"},
		RerankerURL:   "",
		RerankerModel: "",
		SourceRoot:    "",
	}
}

// WithDefaults returns cfg with implicit directory-index defaults applied.
func (c DirectoryIndexesConfig) WithDefaults() DirectoryIndexesConfig {
	if strings.TrimSpace(c.Filename) == "" {
		c.Filename = "INDEX.md"
	}
	if strings.TrimSpace(c.Mode) == "" {
		c.Mode = "managed-block"
	}
	return c
}

// ResolveSourceRoot returns SourceRoot as an absolute filesystem path, resolving
// relative values against vaultRoot. An empty SourceRoot returns an empty path.
func (c Config) ResolveSourceRoot(vaultRoot string) (string, error) {
	raw := strings.TrimSpace(c.SourceRoot)
	if raw == "" {
		return "", nil
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), nil
	}
	abs, err := filepath.Abs(filepath.Join(vaultRoot, filepath.FromSlash(raw)))
	if err != nil {
		return "", fmt.Errorf("resolve source root: %w", err)
	}
	return abs, nil
}

// Load reads and parses config.toml at path, falling back to defaults for any
// unset field.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := Default()
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes cfg to path as TOML with 0600 permissions.
func Save(path string, cfg Config) error {
	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// --- Layout ---

// Layout resolves the standard paths inside a vault's .stardust directory.
type Layout struct {
	Root string // the vault root, which contains .stardust/
}

// Dir returns the .stardust directory path.
func (l Layout) Dir() string { return filepath.Join(l.Root, DirName) }

// Config returns the config.toml path.
func (l Layout) Config() string { return filepath.Join(l.Dir(), "config.toml") }

// SyncConfig returns the agent sync config path.
func (l Layout) SyncConfig() string { return filepath.Join(l.Dir(), "sync.toml") }

// Manifest returns the pinned agent-manifest path.
func (l Layout) Manifest() string { return filepath.Join(l.Dir(), "manifest.md") }

// Rules returns the canonical hand-authored agent-rules source path.
func (l Layout) Rules() string { return filepath.Join(l.Dir(), "rules.md") }

// IndexMD returns the generated table-of-contents path.
func (l Layout) IndexMD() string { return filepath.Join(l.Dir(), "INDEX.md") }

// Baseline returns the committed CI-ratchet baseline path.
func (l Layout) Baseline() string { return filepath.Join(l.Dir(), "baseline.json") }

// Cache returns the gitignored derived-cache directory.
func (l Layout) Cache() string { return filepath.Join(l.Dir(), "cache") }

// DB returns the sqlite index path.
func (l Layout) DB() string { return filepath.Join(l.Cache(), "db.sqlite") }

// GraphJSON returns the derived link-graph path.
func (l Layout) GraphJSON() string { return filepath.Join(l.Cache(), "graph.json") }

// Hooks returns the versioned git-hooks directory.
func (l Layout) Hooks() string { return filepath.Join(l.Dir(), "hooks") }

// Mounts returns the external-source connector directory.
func (l Layout) Mounts() string { return filepath.Join(l.Dir(), "mounts") }

// CronJobs returns the declarative cron-jobs directory.
func (l Layout) CronJobs() string { return filepath.Join(l.Dir(), "cron-jobs") }

// Collections returns the collection-schema directory.
func (l Layout) Collections() string { return filepath.Join(l.Dir(), "collections") }

// --- Root resolution ---

// ErrNoVault indicates no .stardust directory was found walking up from a start path.
var ErrNoVault = errors.New("config: no .stardust directory found (run 'stardust init')")

// FindRoot walks up from start until it finds a directory containing .stardust,
// returning that directory. It returns ErrNoVault if none is found.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start path: %w", err)
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, DirName)); err == nil && fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoVault
		}
		dir = parent
	}
}
