package agentsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rulesSource = "---\nname: rules\ntargets: [claude, codex, gemini]\n---\n" +
	"# House rules\n\nAlways compose, never clobber.\n"

func countRulesMarkers(body string) (starts, ends int) {
	return strings.Count(body, rulesBlockStart), strings.Count(body, rulesBlockEnd)
}

func readRulesFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestRenderStripsFrontmatter(t *testing.T) {
	got := renderRulesBody(rulesSource)
	if strings.Contains(got, "---") {
		t.Fatalf("render kept a frontmatter fence: %q", got)
	}
	if strings.Contains(got, "name: rules") {
		t.Fatalf("render kept frontmatter keys: %q", got)
	}
	if !strings.Contains(got, "# House rules") {
		t.Fatalf("render dropped the body: %q", got)
	}
	if strings.HasSuffix(got, "\n") || strings.HasPrefix(got, "\n") {
		t.Fatalf("render did not trim surrounding whitespace: %q", got)
	}
}

func TestRulesAdaptersCoverEveryTool(t *testing.T) {
	for _, tool := range []Tool{ToolClaude, ToolCodex, ToolGemini} {
		adapter, ok := rulesAdapters[tool]
		if !ok {
			t.Fatalf("no rules adapter for %q", tool)
		}
		if adapter.fileName == "" || adapter.render == nil {
			t.Fatalf("incomplete adapter for %q: %+v", tool, adapter)
		}
	}
	want := map[Tool]string{ToolClaude: "CLAUDE.md", ToolCodex: "AGENTS.md", ToolGemini: "GEMINI.md"}
	for tool, name := range want {
		if got := rulesAdapters[tool].fileName; got != name {
			t.Fatalf("%s fileName = %q, want %q", tool, got, name)
		}
	}
}

func TestInjectRulesBlockCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "CLAUDE.md")

	if err := injectRulesBlock(path, "# House rules"); err != nil {
		t.Fatalf("injectRulesBlock() error = %v", err)
	}
	body := readRulesFile(t, path)
	if starts, ends := countRulesMarkers(body); starts != 1 || ends != 1 {
		t.Fatalf("markers = (%d, %d), want one each", starts, ends)
	}
	if !strings.Contains(body, "# House rules") {
		t.Fatalf("body = %q, want the rendered rules", body)
	}
	if strings.Contains(body, "---") {
		t.Fatalf("body = %q, want no frontmatter fence", body)
	}
}

func TestInjectRulesBlockKeepsUserLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	user := "# My project\n\nHand-written notes the human owns.\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	if err := injectRulesBlock(path, "managed rule one"); err != nil {
		t.Fatalf("injectRulesBlock() error = %v", err)
	}

	body := readRulesFile(t, path)
	if !strings.Contains(body, "Hand-written notes the human owns.") {
		t.Fatalf("body = %q, want the user line preserved", body)
	}
	if !strings.Contains(body, "# My project") {
		t.Fatalf("body = %q, want the user heading preserved", body)
	}
	if starts, ends := countRulesMarkers(body); starts != 1 || ends != 1 {
		t.Fatalf("markers = (%d, %d), want one each", starts, ends)
	}
	// The managed block must come after the user content.
	if strings.Index(body, "Hand-written") > strings.Index(body, rulesBlockStart) {
		t.Fatalf("body = %q, want user lines before the managed block", body)
	}
}

func TestInjectRulesBlockIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	user := "# Repo\n\nuser owned line\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	if err := injectRulesBlock(path, "rule body"); err != nil {
		t.Fatalf("first injectRulesBlock() error = %v", err)
	}
	first := readRulesFile(t, path)
	if err := injectRulesBlock(path, "rule body"); err != nil {
		t.Fatalf("second injectRulesBlock() error = %v", err)
	}
	second := readRulesFile(t, path)

	if first != second {
		t.Fatalf("second inject changed the file:\nfirst:\n%q\nsecond:\n%q", first, second)
	}
	if starts, ends := countRulesMarkers(second); starts != 1 || ends != 1 {
		t.Fatalf("markers = (%d, %d), want one each", starts, ends)
	}
	if !strings.Contains(second, "user owned line") {
		t.Fatalf("body = %q, want the user line preserved", second)
	}
}

func TestInjectRulesBlockReplacesStaleBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GEMINI.md")
	if err := injectRulesBlock(path, "old rules"); err != nil {
		t.Fatalf("first injectRulesBlock() error = %v", err)
	}
	if err := injectRulesBlock(path, "new rules"); err != nil {
		t.Fatalf("second injectRulesBlock() error = %v", err)
	}

	body := readRulesFile(t, path)
	if strings.Contains(body, "old rules") {
		t.Fatalf("body = %q, want stale block replaced", body)
	}
	if !strings.Contains(body, "new rules") {
		t.Fatalf("body = %q, want new block present", body)
	}
	if starts, ends := countRulesMarkers(body); starts != 1 || ends != 1 {
		t.Fatalf("markers = (%d, %d), want one each", starts, ends)
	}
}

func TestInjectRulesBlockHandlesCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	user := "# Repo\r\n\r\nwindows authored line\r\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	if err := injectRulesBlock(path, "rule body"); err != nil {
		t.Fatalf("injectRulesBlock() error = %v", err)
	}
	if err := injectRulesBlock(path, "rule body"); err != nil {
		t.Fatalf("second injectRulesBlock() error = %v", err)
	}

	body := readRulesFile(t, path)
	if !strings.Contains(body, "windows authored line") {
		t.Fatalf("body = %q, want the CRLF user line preserved", body)
	}
	if starts, ends := countRulesMarkers(body); starts != 1 || ends != 1 {
		t.Fatalf("markers = (%d, %d), want one each after CRLF re-inject", starts, ends)
	}
}

func TestStripRulesBlockRemovesOnlyTheBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	user := "# Repo\n\nbefore line\nafter line\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}
	if err := injectRulesBlock(path, "managed rules"); err != nil {
		t.Fatalf("injectRulesBlock() error = %v", err)
	}

	if err := stripRulesBlock(path); err != nil {
		t.Fatalf("stripRulesBlock() error = %v", err)
	}

	body := readRulesFile(t, path)
	if starts, ends := countRulesMarkers(body); starts != 0 || ends != 0 {
		t.Fatalf("markers = (%d, %d), want none after strip", starts, ends)
	}
	if strings.Contains(body, "managed rules") {
		t.Fatalf("body = %q, want the managed rules gone", body)
	}
	for _, want := range []string{"before line", "after line", "# Repo"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body = %q, want the user line %q preserved", body, want)
		}
	}
	if strings.Contains(body, "\n\n\n") {
		t.Fatalf("body = %q, want collapsed blank lines after strip", body)
	}
}

func TestStripRulesBlockMissingFileIsNoError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.md")
	if err := stripRulesBlock(path); err != nil {
		t.Fatalf("stripRulesBlock() on missing file error = %v, want nil", err)
	}
}

func TestExtractRulesBlockRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(path, []byte("# Repo\n\nuser\n"), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}
	body := "line one\nline two"
	if err := injectRulesBlock(path, body); err != nil {
		t.Fatalf("injectRulesBlock() error = %v", err)
	}

	inner, found := extractRulesBlock(readRulesFile(t, path))
	if !found {
		t.Fatal("extractRulesBlock() found = false, want true")
	}
	if strings.TrimSpace(inner) != strings.TrimSpace(body) {
		t.Fatalf("extracted %q, want %q", inner, body)
	}

	if _, found := extractRulesBlock("# Repo\n\nno block here\n"); found {
		t.Fatal("extractRulesBlock() found = true on a file with no block")
	}
}
