package convention

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alxxpersonal/stardust/internal/agentsync"
	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// ConventionIssue is one docs or agent convention lint result.
type ConventionIssue struct {
	Severity string `json:"severity"`
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Detail   string `json:"detail"`
}

var (
	timestampedDocNameRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{4}-[a-z0-9][a-z0-9-]*\.md$`)
	adrDocNameRe         = regexp.MustCompile(`^\d{4}-[a-z0-9][a-z0-9-]*\.md$`)
)

// CheckDocs validates convention docs and forbidden dash characters.
func CheckDocs(root string, ignore []string) ([]ConventionIssue, error) {
	paths, err := vault.Scan(root, ignore)
	if err != nil {
		return nil, err
	}
	var issues []ConventionIssue
	for _, rel := range paths {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		if strings.ContainsRune(string(raw), '\u2014') || strings.ContainsRune(string(raw), '\u2013') {
			issues = append(issues, ConventionIssue{Severity: "error", Kind: "forbidden-dash", Path: rel, Detail: "contains a forbidden unicode dash"})
		}
		docType, ok := docTypeForPath(rel)
		if !ok {
			continue
		}
		issues = append(issues, checkDocFile(root, rel, docType)...)
	}
	return issues, nil
}

// CheckSkills validates skill frontmatter targets.
func CheckSkills(root string) ([]ConventionIssue, error) {
	var issues []ConventionIssue
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".stardust":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if d.Name() != "SKILL.md" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		note, err := vault.Parse(root, rel)
		if err != nil {
			return nil
		}
		_, err = agentsync.ParseTargets(note.Frontmatter, []agentsync.Tool{agentsync.ToolClaude, agentsync.ToolCodex, agentsync.ToolGemini})
		if err != nil {
			issues = append(issues, ConventionIssue{Severity: "error", Kind: "bad-target", Path: rel, Detail: err.Error()})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("check skills: %w", err)
	}
	return issues, nil
}

func checkDocFile(root, rel, docType string) []ConventionIssue {
	var issues []ConventionIssue
	name := filepath.Base(rel)
	if !validDocName(docType, name) {
		issues = append(issues, ConventionIssue{Severity: "error", Kind: "bad-doc-name", Path: rel, Detail: "filename does not match convention"})
	}
	note, err := vault.Parse(root, rel)
	if err != nil {
		return issues
	}
	fm := note.Frontmatter
	for _, field := range []string{"title", "type", "status", "created", "updated"} {
		if _, ok := fm[field]; !ok {
			issues = append(issues, ConventionIssue{Severity: "error", Kind: "missing-doc-field", Path: rel, Detail: field + " is required"})
		}
	}
	if got := fmString(fm, "type"); got != "" && got != docType {
		issues = append(issues, ConventionIssue{Severity: "error", Kind: "bad-doc-type", Path: rel, Detail: fmt.Sprintf("type %q does not match %q", got, docType)})
	}
	status := fmString(fm, "status")
	if status != "" && !DocStatusAllowed(docType, status) {
		issues = append(issues, ConventionIssue{Severity: "error", Kind: "bad-doc-status", Path: rel, Detail: fmt.Sprintf("status %q is not allowed for %s", status, docType)})
	}
	issues = append(issues, checkRelated(root, rel, fm)...)
	governsIssues, matched := checkGoverns(root, rel, fm)
	issues = append(issues, governsIssues...)
	if status == "Implemented" && len(matched) > 0 {
		issues = append(issues, checkStale(root, rel, matched)...)
	}
	return issues
}

func checkRelated(root, rel string, fm map[string]any) []ConventionIssue {
	related, err := StringList(fm, "related")
	if err != nil {
		return []ConventionIssue{{Severity: "error", Kind: "broken-doc-ref", Path: rel, Detail: err.Error()}}
	}
	var issues []ConventionIssue
	for _, target := range related {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err != nil {
			if os.IsNotExist(err) {
				issues = append(issues, ConventionIssue{Severity: "error", Kind: "broken-doc-ref", Path: rel, Detail: target + " does not exist"})
				continue
			}
			issues = append(issues, ConventionIssue{Severity: "error", Kind: "broken-doc-ref", Path: rel, Detail: err.Error()})
		}
	}
	return issues
}

func checkGoverns(root, rel string, fm map[string]any) ([]ConventionIssue, []string) {
	governs, err := StringList(fm, "governs")
	if err != nil {
		return []ConventionIssue{{Severity: "error", Kind: "governs-no-match", Path: rel, Detail: err.Error()}}, nil
	}
	var issues []ConventionIssue
	var matched []string
	for _, pattern := range governs {
		files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			issues = append(issues, ConventionIssue{Severity: "error", Kind: "governs-no-match", Path: rel, Detail: err.Error()})
			continue
		}
		if len(files) == 0 {
			issues = append(issues, ConventionIssue{Severity: "error", Kind: "governs-no-match", Path: rel, Detail: pattern + " matches no files"})
			continue
		}
		for _, file := range files {
			relFile, err := filepath.Rel(root, file)
			if err == nil {
				matched = append(matched, filepath.ToSlash(relFile))
			}
		}
	}
	return issues, matched
}

func checkStale(root, rel string, matched []string) []ConventionIssue {
	ctx := context.Background()
	docCommit, err := gitx.LastCommit(ctx, root, rel)
	if err != nil || docCommit == "" {
		return nil
	}
	count, err := gitx.CommitCountSince(ctx, root, docCommit, matched...)
	if err != nil || count == 0 {
		return nil
	}
	return []ConventionIssue{{Severity: "warn", Kind: "stale-governed-doc", Path: rel, Detail: fmt.Sprintf("%d governed code commit(s) since doc update", count)}}
}

// DocTypeForPath returns the convention doc type ("spec", "plan", "adr",
// "research") for a vault-relative path under docs/, and false when the path is
// not a governed doc folder.
func DocTypeForPath(rel string) (string, bool) {
	return docTypeForPath(rel)
}

func docTypeForPath(rel string) (string, bool) {
	rel = filepath.ToSlash(rel)
	switch {
	case strings.HasPrefix(rel, "docs/specs/"):
		return "spec", true
	case strings.HasPrefix(rel, "docs/plans/"):
		return "plan", true
	case strings.HasPrefix(rel, "docs/adr/"):
		return "adr", true
	case strings.HasPrefix(rel, "docs/research/"):
		return "research", true
	default:
		return "", false
	}
}

func validDocName(docType, name string) bool {
	if docType == "adr" {
		return adrDocNameRe.MatchString(name)
	}
	return timestampedDocNameRe.MatchString(name)
}

func fmString(fm map[string]any, key string) string {
	if v, ok := fm[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
