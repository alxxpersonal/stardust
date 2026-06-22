package convention

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDocsReportsConventionIssues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/specs/bad-name.md", "---\ntitle: Bad\ntype: spec\nstatus: Weird\ncreated: 2026-06-22\nupdated: 2026-06-22\nrelated: [\"docs/adr/0001-missing.md\"]\ngoverns: [\"internal/missing/*.go\"]\n---\n# Bad\n"+string(rune(0x2014))+"\n")

	issues, err := CheckDocs(root, nil)
	if err != nil {
		t.Fatalf("CheckDocs() error = %v", err)
	}

	for _, kind := range []string{"bad-doc-name", "bad-doc-status", "forbidden-dash", "broken-doc-ref", "governs-no-match"} {
		if !hasConventionIssue(issues, kind) {
			t.Fatalf("CheckDocs() missing %s in %#v", kind, issues)
		}
	}
}

func TestCheckSkillsReportsBadTargets(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "skills/foo/SKILL.md", "---\nname: foo\ntargets: [wat]\n---\n# Foo\n")

	issues, err := CheckSkills(root)
	if err != nil {
		t.Fatalf("CheckSkills() error = %v", err)
	}
	if !hasConventionIssue(issues, "bad-target") {
		t.Fatalf("CheckSkills() missing bad-target in %#v", issues)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func hasConventionIssue(issues []ConventionIssue, kind string) bool {
	for _, issue := range issues {
		if issue.Kind == kind {
			return true
		}
	}
	return false
}
