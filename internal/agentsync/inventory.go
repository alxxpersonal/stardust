package agentsync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kind identifies the kind of agent asset.
type Kind string

// Supported agent asset kinds.
const (
	KindSkill Kind = "skill"
	KindAgent Kind = "agent"
)

// Item is one discovered agent asset routed to one or more tools.
type Item struct {
	Name        string         `json:"name"`
	Kind        Kind           `json:"kind"`
	SourcePath  string         `json:"source_path"`
	Frontmatter map[string]any `json:"frontmatter"`
	Targets     []Tool         `json:"targets"`
	Hash        string         `json:"hash"`
	Source      Source         `json:"source"`
}

// Discover scans configured sources for skills and agents.
func Discover(cfg Config) ([]Item, error) {
	chosen := map[string]Item{}
	for _, src := range cfg.Sources {
		items, err := discoverSource(src, cfg.DefaultTargets)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			key := string(item.Kind) + "\x00" + item.Name
			current, ok := chosen[key]
			if !ok || item.Source.Priority < current.Source.Priority {
				chosen[key] = item
			}
		}
	}

	out := make([]Item, 0, len(chosen))
	for _, item := range chosen {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ParseTargets returns the explicitly routed tools or the configured defaults.
func ParseTargets(frontmatter map[string]any, defaults []Tool) ([]Tool, error) {
	raw, ok := frontmatter["targets"]
	if !ok {
		return cloneTools(defaults), nil
	}
	switch list := raw.(type) {
	case []any:
		return parseTargetList(list)
	case []string:
		items := make([]any, 0, len(list))
		for _, item := range list {
			items = append(items, item)
		}
		return parseTargetList(items)
	default:
		return nil, fmt.Errorf("targets must be a list of strings")
	}
}

func discoverSource(src Source, defaults []Tool) ([]Item, error) {
	if _, err := os.Stat(src.Path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat source %s: %w", src.Name, err)
	}
	switch src.Kind {
	case string(KindSkill):
		return discoverSkills(src, defaults)
	case string(KindAgent):
		return discoverAgents(src, defaults)
	default:
		return nil, fmt.Errorf("source %s: unsupported kind %q", src.Name, src.Kind)
	}
}

func discoverSkills(src Source, defaults []Tool) ([]Item, error) {
	var items []Item
	err := filepath.WalkDir(src.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		skillPath := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("stat skill %s: %w", skillPath, err)
		}
		item, err := readItem(src, KindSkill, path, skillPath)
		if err != nil {
			return err
		}
		targets, err := ParseTargets(item.Frontmatter, defaults)
		if err != nil {
			return fmt.Errorf("skill %s: %w", item.Name, err)
		}
		item.Targets = targets
		items = append(items, item)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("discover skills %s: %w", src.Name, err)
	}
	return items, nil
}

func discoverAgents(src Source, defaults []Tool) ([]Item, error) {
	var items []Item
	err := filepath.WalkDir(src.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		item, err := readItem(src, KindAgent, path, path)
		if err != nil {
			return err
		}
		targets, err := ParseTargets(item.Frontmatter, defaults)
		if err != nil {
			return fmt.Errorf("agent %s: %w", item.Name, err)
		}
		item.Targets = targets
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover agents %s: %w", src.Name, err)
	}
	return items, nil
}

func readItem(src Source, kind Kind, sourcePath, contentPath string) (Item, error) {
	raw, err := os.ReadFile(contentPath)
	if err != nil {
		return Item{}, fmt.Errorf("read %s: %w", contentPath, err)
	}
	frontmatter, err := parseFrontmatter(raw)
	if err != nil {
		return Item{}, fmt.Errorf("parse frontmatter %s: %w", contentPath, err)
	}
	name := frontmatterString(frontmatter, "name")
	if name == "" {
		name = itemName(kind, sourcePath)
	}
	return Item{
		Name:        name,
		Kind:        kind,
		SourcePath:  filepath.Clean(sourcePath),
		Frontmatter: frontmatter,
		Hash:        hashBytes(raw),
		Source:      src,
	}, nil
}

func parseFrontmatter(raw []byte) (map[string]any, error) {
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return map[string]any{}, nil
	}
	rest := text[4:]
	if strings.HasPrefix(text, "---\r\n") {
		rest = text[5:]
	}
	for _, marker := range []string{"\n---\n", "\n---\r\n", "\r\n---\r\n", "\r\n---\n"} {
		if idx := strings.Index(rest, marker); idx >= 0 {
			fm := map[string]any{}
			if err := yaml.Unmarshal([]byte(rest[:idx]), &fm); err != nil {
				return nil, err
			}
			if fm == nil {
				fm = map[string]any{}
			}
			return fm, nil
		}
	}
	return nil, fmt.Errorf("missing closing frontmatter marker")
}

func parseTargetList(list []any) ([]Tool, error) {
	if len(list) == 0 {
		return nil, fmt.Errorf("targets cannot be empty")
	}
	targets := make([]Tool, 0, len(list))
	for i, raw := range list {
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("targets[%d] must be a string", i)
		}
		tool := Tool(strings.TrimSpace(s))
		if err := validateTool(tool); err != nil {
			return nil, err
		}
		targets = append(targets, tool)
	}
	return targets, nil
}

func cloneTools(in []Tool) []Tool {
	out := make([]Tool, len(in))
	copy(out, in)
	return out
}

func frontmatterString(frontmatter map[string]any, key string) string {
	if v, ok := frontmatter[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func itemName(kind Kind, sourcePath string) string {
	if kind == KindAgent {
		return strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}
	return filepath.Base(sourcePath)
}

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
