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

// TestCheckDocFileDefaultSchemaFires asserts that with no committed collection
// config the default schema still enforces required fields and the status enum.
func TestCheckDocFileDefaultSchemaFires(t *testing.T) {
	root := t.TempDir()
	// missing created, invalid status
	writeFile(t, root, "docs/specs/2026-06-22-1000-probe.md", "---\ntitle: Probe\ntype: spec\nstatus: Bogus\nupdated: 2026-06-22\n---\n# Probe\n")

	issues, err := CheckDocs(root, nil)
	if err != nil {
		t.Fatalf("CheckDocs() error = %v", err)
	}
	if !hasIssueDetail(issues, "missing-doc-field", "created is required") {
		t.Fatalf("expected missing created via schema, got %#v", issues)
	}
	if !hasConventionIssue(issues, "bad-doc-status") {
		t.Fatalf("expected bad-doc-status via schema, got %#v", issues)
	}
}

// TestCheckDocFileUsesCommittedSchema asserts the checker validates against the
// committed per-collection schema (collections.Validate), enforcing a custom
// required field a hardcoded set would never know about.
func TestCheckDocFileUsesCommittedSchema(t *testing.T) {
	root := t.TempDir()
	cfg := "path = \"docs/specs\"\ndescription = \"specs\"\n\n" +
		"[[fields]]\nname = \"title\"\ntype = \"string\"\nrequired = true\n\n" +
		"[[fields]]\nname = \"owner\"\ntype = \"string\"\nrequired = true\n"
	writeFile(t, root, ".stardust/collections/specs/config.toml", cfg)
	writeFile(t, root, "docs/specs/2026-06-22-1000-probe.md", "---\ntitle: Probe\n---\n# Probe\n")

	issues, err := CheckDocs(root, nil)
	if err != nil {
		t.Fatalf("CheckDocs() error = %v", err)
	}
	if !hasIssueDetail(issues, "missing-doc-field", "owner is required") {
		t.Fatalf("expected committed-schema owner field enforced, got %#v", issues)
	}
}

func TestCheckDocsReportsStrayDocs(t *testing.T) {
	root := t.TempDir()
	cfg := "path = \"docs/specs\"\ndescription = \"specs\"\n\n" +
		"[[fields]]\nname = \"title\"\ntype = \"string\"\nrequired = true\n"
	writeFile(t, root, ".stardust/collections/specs/config.toml", cfg)
	writeFile(t, root, "docs/specs/2026-06-22-1000-good.md", "---\ntitle: Good\n---\n# Good\n")
	writeFile(t, root, "docs/superpowers/mirror.md", "# Mirror\n")
	writeFile(t, root, "docs/loose.md", "# Loose\n")
	writeFile(t, root, "docs/INDEX.md", "# Docs Index\n")
	writeFile(t, root, "docs/templates/spec.md", "# Template\n")

	issues, err := CheckDocs(root, nil)
	if err != nil {
		t.Fatalf("CheckDocs() error = %v", err)
	}

	if !hasIssuePath(issues, "stray-doc", "docs/superpowers/mirror.md") {
		t.Fatalf("expected stray-doc for mirror folder, got %#v", issues)
	}
	if !hasIssuePath(issues, "stray-doc", "docs/loose.md") {
		t.Fatalf("expected stray-doc for loose doc, got %#v", issues)
	}
	if hasIssuePath(issues, "stray-doc", "docs/INDEX.md") {
		t.Fatalf("docs/INDEX.md must be exempt, got %#v", issues)
	}
	if hasIssuePath(issues, "stray-doc", "docs/templates/spec.md") {
		t.Fatalf("docs/templates must be exempt, got %#v", issues)
	}
	if hasIssuePath(issues, "stray-doc", "docs/specs/2026-06-22-1000-good.md") {
		t.Fatalf("registered collection doc must be allowed, got %#v", issues)
	}
}

func hasIssuePath(issues []ConventionIssue, kind, path string) bool {
	for _, issue := range issues {
		if issue.Kind == kind && issue.Path == path {
			return true
		}
	}
	return false
}

func hasIssueDetail(issues []ConventionIssue, kind, detail string) bool {
	for _, issue := range issues {
		if issue.Kind == kind && issue.Detail == detail {
			return true
		}
	}
	return false
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
