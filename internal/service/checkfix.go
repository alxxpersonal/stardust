package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alxxpersonal/stardust/internal/convention"
	"github.com/alxxpersonal/stardust/internal/memory"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// Fix records one mechanically-safe edit CheckFix applied to a doc.
type Fix struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// FixResult is the outcome of an autofix pass over convention docs.
type FixResult struct {
	Fixes    []Fix  `json:"fixes"`
	Fixed    int    `json:"fixed"`
	Markdown string `json:"markdown"`
}

// fixableKinds are the issue kinds CheckFix knows how to repair without judgment.
// Everything else (bad-doc-status, broken-doc-ref, governs-no-match, bad-doc-name,
// and the missing-doc-field cases for title/status) is left reported-only.
var fixableKinds = map[string]bool{
	"forbidden-dash":    true,
	"missing-doc-field": true,
	"bad-doc-type":      true,
}

// CheckFix autofixes only the mechanically-safe convention issues that CheckDocs
// emits: forbidden unicode dashes become hyphen-minus, a missing or wrong type is
// derived from the doc folder, and missing created/updated dates are backfilled
// from the file modification time. Ambiguous issues (bad status, broken refs,
// unmatched governs globs, missing title/status) are never touched. It returns
// what it changed; re-running Check afterward reports any remaining issues.
func (s *Service) CheckFix(ctx context.Context) (FixResult, error) {
	issues, err := convention.CheckDocs(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return FixResult{}, err
	}

	// Group fixable issues per path so each file is read and rewritten once.
	dashPaths := map[string]bool{}
	fieldPaths := map[string]bool{}
	for _, issue := range issues {
		if !fixableKinds[issue.Kind] {
			continue
		}
		switch issue.Kind {
		case "forbidden-dash":
			dashPaths[issue.Path] = true
		case "bad-doc-type":
			fieldPaths[issue.Path] = true
		case "missing-doc-field":
			if field := missingFieldName(issue.Detail); field == "type" || field == "created" || field == "updated" {
				fieldPaths[issue.Path] = true
			}
		}
	}

	mem := memory.New(s.Layout.Root)
	var fixes []Fix

	paths := mergedSortedPaths(dashPaths, fieldPaths)
	for _, rel := range paths {
		if dashPaths[rel] {
			fixed, err := s.fixDashes(mem, rel)
			if err != nil {
				return FixResult{}, err
			}
			if fixed {
				fixes = append(fixes, Fix{Path: rel, Kind: "forbidden-dash", Detail: "replaced forbidden unicode dashes with hyphen-minus"})
			}
		}
		if fieldPaths[rel] {
			applied, err := s.fixDocFields(ctx, mem, rel)
			if err != nil {
				return FixResult{}, err
			}
			fixes = append(fixes, applied...)
		}
	}

	res := FixResult{Fixes: fixes, Fixed: len(fixes)}
	res.Markdown = renderFix(res)
	return res, nil
}

// fixDashes rewrites U+2014 and U+2013 to hyphen-minus across the whole file
// (frontmatter and body) through the path-confined memory store.
func (s *Service) fixDashes(mem *memory.Store, rel string) (bool, error) {
	content, err := mem.View(rel)
	if err != nil {
		return false, err
	}
	replaced := strings.NewReplacer("—", "-", "–", "-").Replace(content)
	if replaced == content {
		return false, nil
	}
	if err := rewriteFile(mem, rel, replaced); err != nil {
		return false, err
	}
	return true, nil
}

// fixDocFields backfills a missing or mismatched type from the doc folder and
// missing created/updated dates from the file modification time, rewriting the
// note through the memory store while preserving its body.
func (s *Service) fixDocFields(_ context.Context, mem *memory.Store, rel string) ([]Fix, error) {
	docType, ok := convention.DocTypeForPath(rel)
	if !ok {
		return nil, nil
	}
	note, err := vault.Parse(s.Layout.Root, filepath.ToSlash(rel))
	if err != nil {
		return nil, nil
	}
	fm := map[string]any{}
	for k, v := range note.Frontmatter {
		fm[k] = v
	}

	var fixes []Fix
	if got := fmStringValue(fm, "type"); got != docType {
		fm["type"] = docType
		if got == "" {
			fixes = append(fixes, Fix{Path: rel, Kind: "missing-doc-field", Detail: "set type to " + docType})
		} else {
			fixes = append(fixes, Fix{Path: rel, Kind: "bad-doc-type", Detail: fmt.Sprintf("rewrote type from %q to %q", got, docType)})
		}
	}

	date := fileDate(filepath.Join(s.Layout.Root, filepath.FromSlash(rel)))
	for _, field := range []string{"created", "updated"} {
		if fmStringValue(fm, field) == "" {
			fm[field] = date
			fixes = append(fixes, Fix{Path: rel, Kind: "missing-doc-field", Detail: "set " + field + " to " + date})
		}
	}

	if len(fixes) == 0 {
		return nil, nil
	}

	content, err := composeNote(fm, note.Body)
	if err != nil {
		return nil, err
	}
	if err := rewriteFile(mem, rel, content); err != nil {
		return nil, err
	}
	return fixes, nil
}

// rewriteFile replaces a file's contents in place through the memory store's
// delete-then-create idiom, keeping the write path-confined and serialized.
func rewriteFile(mem *memory.Store, rel, content string) error {
	if err := mem.Delete(rel); err != nil {
		return err
	}
	return mem.Create(rel, content)
}

// fileDate returns the file's modification date as a YYYY-MM-DD string, matching
// the date format new docs are scaffolded with.
func fileDate(abs string) string {
	if info, err := os.Stat(abs); err == nil {
		return info.ModTime().UTC().Format("2006-01-02")
	}
	return time.Now().UTC().Format("2006-01-02")
}

// missingFieldName extracts the field name from a missing-doc-field detail, which
// CheckDocs renders as "<field> is required".
func missingFieldName(detail string) string {
	fields := strings.Fields(detail)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func fmStringValue(fm map[string]any, key string) string {
	if v, ok := fm[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func mergedSortedPaths(a, b map[string]bool) []string {
	seen := map[string]bool{}
	var paths []string
	for _, m := range []map[string]bool{a, b} {
		for p := range m {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

func renderFix(res FixResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Check fix\n\n%d fix(es) applied.\n\n", res.Fixed)
	if res.Fixed == 0 {
		b.WriteString("Nothing to autofix.\n")
		return b.String()
	}
	for _, f := range res.Fixes {
		fmt.Fprintf(&b, "- [%s] `%s` - %s\n", f.Kind, f.Path, f.Detail)
	}
	return b.String()
}
