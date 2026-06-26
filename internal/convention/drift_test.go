package convention

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestCheckDocsDriftFlagsReferencedCode asserts that a doc which references code
// through related: and an inline path is flagged with a drift warning naming the
// moved file and the commit count, ungated by status, while a doc whose
// referenced code has not moved is left alone.
func TestCheckDocsDriftFlagsReferencedCode(t *testing.T) {
	root := t.TempDir()
	gitInitConvention(t, root)

	// a code file the ADR will reference, plus a second untouched file.
	writeFile(t, root, "internal/store/daemon.go", "package store\n")
	writeFile(t, root, "internal/store/quiet.go", "package store\n")

	// an ADR that points at daemon.go through related: and an inline path, with a
	// status that is not Implemented, so only ungated reference binding can flag it.
	writeFile(t, root, "docs/adr/0001-uses-daemon.md",
		"---\ntitle: Uses Daemon\ntype: adr\nstatus: Proposed\ncreated: 2026-06-26\nupdated: 2026-06-26\n"+
			"related: [\"internal/store/daemon.go\"]\n---\n# Uses Daemon\n\nThe store lives in `internal/store/daemon.go` and stays quiet in `internal/store/quiet.go`.\n")
	gitCommitConvention(t, root, "2026-06-26T10:00:00", "add adr and code")

	// move daemon.go twice after the ADR's last commit; quiet.go never moves.
	writeFile(t, root, "internal/store/daemon.go", "package store\n\nconst A = 1\n")
	gitCommitConvention(t, root, "2026-06-27T10:00:00", "edit daemon 1")
	writeFile(t, root, "internal/store/daemon.go", "package store\n\nconst A = 2\n")
	gitCommitConvention(t, root, "2026-06-28T10:00:00", "edit daemon 2")

	issues, err := CheckDocs(root, nil)
	if err != nil {
		t.Fatalf("CheckDocs() error = %v", err)
	}

	var drift *ConventionIssue
	for i := range issues {
		if issues[i].Kind == "drift" {
			drift = &issues[i]
			break
		}
	}
	if drift == nil {
		t.Fatalf("expected a drift issue, got %#v", issues)
	}
	if drift.Severity != "warn" {
		t.Fatalf("drift severity = %q, want warn", drift.Severity)
	}
	if !strings.Contains(drift.Detail, "internal/store/daemon.go") {
		t.Fatalf("drift detail does not name the file: %q", drift.Detail)
	}
	if !strings.Contains(drift.Detail, "2") {
		t.Fatalf("drift detail does not carry the commit count: %q", drift.Detail)
	}
	if !strings.Contains(drift.Detail, "review") {
		t.Fatalf("drift detail is not phrased as a review prompt: %q", drift.Detail)
	}
	// quiet.go never moved, so it must not drift.
	for _, is := range issues {
		if is.Kind == "drift" && strings.Contains(is.Detail, "quiet.go") {
			t.Fatalf("quiet.go should not drift: %q", is.Detail)
		}
	}
}

// TestCheckDocsDriftCleanWhenCodeUnmoved asserts no drift fires when the
// referenced code carries no commits since the doc's last touch.
func TestCheckDocsDriftCleanWhenCodeUnmoved(t *testing.T) {
	root := t.TempDir()
	gitInitConvention(t, root)
	writeFile(t, root, "internal/store/daemon.go", "package store\n")
	writeFile(t, root, "docs/adr/0001-uses-daemon.md",
		"---\ntitle: Uses Daemon\ntype: adr\nstatus: Proposed\ncreated: 2026-06-26\nupdated: 2026-06-26\n"+
			"related: [\"internal/store/daemon.go\"]\n---\n# Uses Daemon\n\nSee `internal/store/daemon.go`.\n")
	gitCommitConvention(t, root, "2026-06-26T10:00:00", "add adr and code")

	issues, err := CheckDocs(root, nil)
	if err != nil {
		t.Fatalf("CheckDocs() error = %v", err)
	}
	for _, is := range issues {
		if is.Kind == "drift" {
			t.Fatalf("expected no drift, got %q", is.Detail)
		}
	}
}

// TestCheckDocsGovernsKeepsImplementedGate asserts the governs: path is unchanged:
// a governs-bound doc that is not Implemented produces neither stale-governed-doc
// nor a reference drift (governs is not a reference binding).
func TestCheckDocsGovernsKeepsImplementedGate(t *testing.T) {
	root := t.TempDir()
	gitInitConvention(t, root)
	writeFile(t, root, "internal/foo.go", "package internal\n")
	writeFile(t, root, "docs/specs/2026-06-26-1000-governs-spec.md",
		"---\ntitle: Governs Spec\ntype: spec\nstatus: Draft\ncreated: 2026-06-26\nupdated: 2026-06-26\n"+
			"governs: [\"internal/foo.go\"]\n---\n# Governs Spec\n")
	gitCommitConvention(t, root, "2026-06-26T10:00:00", "add spec and code")
	writeFile(t, root, "internal/foo.go", "package internal\n\nconst X = 1\n")
	gitCommitConvention(t, root, "2026-06-27T10:00:00", "edit foo")

	issues, err := CheckDocs(root, nil)
	if err != nil {
		t.Fatalf("CheckDocs() error = %v", err)
	}
	for _, is := range issues {
		if is.Kind == "stale-governed-doc" {
			t.Fatalf("governs gate should suppress stale-governed-doc for a Draft doc: %q", is.Detail)
		}
		if is.Kind == "drift" {
			t.Fatalf("governs is not a reference binding and must not drift: %q", is.Detail)
		}
	}
}

func gitInitConvention(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	} {
		runGit(t, root, args...)
	}
}

func gitCommitConvention(t *testing.T, root, date, msg string) {
	t.Helper()
	runGit(t, root, "add", "-A")
	cmd := exec.Command("git", "-C", root, "commit", "-m", msg)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, string(out))
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(out))
	}
}
