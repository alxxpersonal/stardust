package agentsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTargetsUsesDefaultsWhenAbsent(t *testing.T) {
	got, err := ParseTargets(nil, []Tool{ToolClaude, ToolGemini})
	if err != nil {
		t.Fatalf("ParseTargets() error = %v", err)
	}
	want := []Tool{ToolClaude, ToolGemini}
	if !sameTools(got, want) {
		t.Fatalf("ParseTargets() = %#v, want %#v", got, want)
	}
}

func TestParseTargetsUsesExplicitTargets(t *testing.T) {
	got, err := ParseTargets(map[string]any{"targets": []any{"claude", "codex"}}, []Tool{ToolGemini})
	if err != nil {
		t.Fatalf("ParseTargets() error = %v", err)
	}
	want := []Tool{ToolClaude, ToolCodex}
	if !sameTools(got, want) {
		t.Fatalf("ParseTargets() = %#v, want %#v", got, want)
	}
}

func TestParseTargetsRejectsUnknownTargets(t *testing.T) {
	_, err := ParseTargets(map[string]any{"targets": []any{"wat"}}, []Tool{ToolClaude})
	if err == nil {
		t.Fatal("ParseTargets() error = nil, want validation error")
	}
}

func TestDiscoverRoutesSkillsByFrontmatterTargets(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "src")
	writeSkill(t, source, "foo", "---\nname: foo\ndescription: Foo skill\n---\n# Foo\n")
	writeSkill(t, source, "bar", "---\nname: bar\ntargets: [claude, codex]\n---\n# Bar\n")

	items, err := Discover(Config{
		Sources:        []Source{{Name: "canonical", Path: source, Kind: "skill", Priority: 10}},
		DefaultTargets: []Tool{ToolClaude, ToolGemini},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("Discover() returned %d items, want 2: %#v", len(items), items)
	}

	byName := map[string]Item{}
	for _, item := range items {
		byName[item.Name] = item
	}
	if got, want := byName["foo"].Targets, []Tool{ToolClaude, ToolGemini}; !sameTools(got, want) {
		t.Fatalf("foo targets = %#v, want %#v", got, want)
	}
	if got, want := byName["bar"].Targets, []Tool{ToolClaude, ToolCodex}; !sameTools(got, want) {
		t.Fatalf("bar targets = %#v, want %#v", got, want)
	}
	if got, want := byName["foo"].Kind, KindSkill; got != want {
		t.Fatalf("foo kind = %q, want %q", got, want)
	}
	if byName["foo"].Hash == "" {
		t.Fatal("foo hash is empty")
	}
}

func TestDiscoverRejectsUnknownTargets(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "src")
	writeSkill(t, source, "bad", "---\nname: bad\ntargets: [wat]\n---\n# Bad\n")

	_, err := Discover(Config{
		Sources:        []Source{{Name: "canonical", Path: source, Kind: "skill", Priority: 10}},
		DefaultTargets: []Tool{ToolClaude},
	})
	if err == nil {
		t.Fatal("Discover() error = nil, want validation error")
	}
}

func TestDiscoverChoosesLowerPriorityDuplicate(t *testing.T) {
	root := t.TempDir()
	high := filepath.Join(root, "high")
	low := filepath.Join(root, "low")
	writeSkill(t, high, "same", "---\nname: shared\n---\n# High\n")
	writeSkill(t, low, "same", "---\nname: shared\n---\n# Low\n")

	items, err := Discover(Config{
		Sources: []Source{
			{Name: "high", Path: high, Kind: "skill", Priority: 20},
			{Name: "low", Path: low, Kind: "skill", Priority: 5},
		},
		DefaultTargets: []Tool{ToolClaude},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Discover() returned %d items, want 1: %#v", len(items), items)
	}
	if got, want := items[0].Source.Name, "low"; got != want {
		t.Fatalf("chosen source = %q, want %q", got, want)
	}
}

func TestDiscoverFindsAgentMarkdownFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "agents")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "reviewer.md"), []byte("---\nname: reviewer\n---\n# Reviewer\n"), 0o644); err != nil {
		t.Fatalf("write agent: %v", err)
	}

	items, err := Discover(Config{
		Sources:        []Source{{Name: "agents", Path: source, Kind: "agent", Priority: 10}},
		DefaultTargets: []Tool{ToolCodex},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Discover() returned %d items, want 1: %#v", len(items), items)
	}
	if got, want := items[0].Kind, KindAgent; got != want {
		t.Fatalf("agent kind = %q, want %q", got, want)
	}
	if got, want := items[0].Name, "reviewer"; got != want {
		t.Fatalf("agent name = %q, want %q", got, want)
	}
}

func writeSkill(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func sameTools(a, b []Tool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
