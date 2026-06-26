package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/convention"
	"github.com/alxxpersonal/stardust/internal/gitx"
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

// fieldDerivations are the schema field names CheckFix can mechanically derive:
// type from the doc folder, created and updated from git history or mtime.
var fieldDerivations = map[string]bool{
	"type":    true,
	"created": true,
	"updated": true,
}

// fixableDocFields returns the subset of schema field names CheckFix can repair:
// a required field that also has a derivation rule. title and status have no safe
// derivation and stay report-only, so fixability is a projection of the schema.
func fixableDocFields(fields []collections.Field) map[string]bool {
	out := map[string]bool{}
	for _, f := range fields {
		if f.Required && fieldDerivations[f.Name] {
			out[f.Name] = true
		}
	}
	return out
}

// fieldFixable reports whether a missing doc field can be mechanically repaired,
// derived from the owning collection's schema.
func (s *Service) fieldFixable(rel, field string) bool {
	docType, ok := convention.DocTypeForPath(rel)
	if !ok {
		return false
	}
	fields, ok := convention.DocFields(s.Layout.Root, docType)
	if !ok {
		return false
	}
	return fixableDocFields(fields)[field]
}

// CheckFix autofixes only the mechanically-safe convention issues that CheckDocs
// emits: forbidden unicode dashes become hyphen-minus, a missing or wrong type is
// derived from the doc folder, missing created/updated dates are backfilled from
// git history (mtime when untracked), and an off-convention filename is renamed
// to its convention name via git mv. Ambiguous issues (bad status, broken refs,
// unmatched governs globs, missing title/status) are never touched. It returns
// what it changed; re-running Check afterward reports any remaining issues.
func (s *Service) CheckFix(ctx context.Context) (FixResult, error) {
	issues, err := convention.CheckDocs(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return FixResult{}, err
	}

	// Group fixable issues per path so each file is read and rewritten once.
	// Fixability is schema-derived: a missing required field is only grouped when
	// its collection schema gives it a derivation rule.
	dashPaths := map[string]bool{}
	fieldPaths := map[string]bool{}
	namePaths := map[string]bool{}
	for _, issue := range issues {
		switch issue.Kind {
		case "forbidden-dash":
			dashPaths[issue.Path] = true
		case "bad-doc-type":
			fieldPaths[issue.Path] = true
		case "bad-doc-name":
			namePaths[issue.Path] = true
		case "missing-doc-field":
			if s.fieldFixable(issue.Path, missingFieldName(issue.Detail)) {
				fieldPaths[issue.Path] = true
			}
		}
	}

	mem := memory.New(s.Layout.Root)
	var fixes []Fix

	// Content fixes (dashes, fields) run on the original path before any rename,
	// then the rename moves the already-repaired file to its convention name.
	paths := mergedSortedPaths(dashPaths, fieldPaths, namePaths)
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
		if namePaths[rel] {
			_, fix, err := s.fixDocName(ctx, mem, rel)
			if err != nil {
				return FixResult{}, err
			}
			if fix != nil {
				fixes = append(fixes, *fix)
			}
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
	replaced := strings.NewReplacer("\u2014", "-", "\u2013", "-").Replace(content)
	if replaced == content {
		return false, nil
	}
	if err := rewriteFile(mem, rel, replaced); err != nil {
		return false, err
	}
	return true, nil
}

// fixDocFields backfills a missing or mismatched type from the doc folder and
// missing created/updated dates from git history (the first and last commit
// touching the file), falling back to the file modification time for untracked
// or non-repo files, rewriting the note through the memory store while
// preserving its body.
func (s *Service) fixDocFields(ctx context.Context, mem *memory.Store, rel string) ([]Fix, error) {
	docType, ok := convention.DocTypeForPath(rel)
	if !ok {
		return nil, nil
	}
	fields, ok := convention.DocFields(s.Layout.Root, docType)
	if !ok {
		return nil, nil
	}
	fixable := fixableDocFields(fields)
	note, err := vault.Parse(s.Layout.Root, filepath.ToSlash(rel))
	if err != nil {
		return nil, nil
	}
	fm := map[string]any{}
	for k, v := range note.Frontmatter {
		fm[k] = v
	}

	var fixes []Fix
	if fixable["type"] {
		if got := fmStringValue(fm, "type"); got != docType {
			fm["type"] = docType
			if got == "" {
				fixes = append(fixes, Fix{Path: rel, Kind: "missing-doc-field", Detail: "set type to " + docType})
			} else {
				fixes = append(fixes, Fix{Path: rel, Kind: "bad-doc-type", Detail: fmt.Sprintf("rewrote type from %q to %q", got, docType)})
			}
		}
	}

	dates := map[string]string{
		"created": s.docFirstDate(ctx, rel),
		"updated": s.docLastDate(ctx, rel),
	}
	for _, field := range []string{"created", "updated"} {
		if fixable[field] && fmStringValue(fm, field) == "" {
			fm[field] = dates[field]
			fixes = append(fixes, Fix{Path: rel, Kind: "missing-doc-field", Detail: "set " + field + " to " + dates[field]})
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

// fixDocName renames an off-convention doc file to its convention name,
// preserving git history with git mv when the file is tracked and falling back
// to a plain rename otherwise. It reindexes the old (pruned) and new paths and
// returns the new vault-relative path plus the applied fix, or the unchanged
// path and a nil fix when nothing was renamed.
func (s *Service) fixDocName(ctx context.Context, mem *memory.Store, rel string) (string, *Fix, error) {
	docType, ok := convention.DocTypeForPath(rel)
	if !ok {
		return rel, nil, nil
	}
	newRel, err := s.conventionDocName(ctx, docType, rel)
	if err != nil {
		return rel, nil, err
	}
	if newRel == "" || newRel == rel {
		return rel, nil, nil
	}
	root := s.Layout.Root
	if gitx.IsRepo(ctx, root) && gitx.IsTracked(ctx, root, rel) {
		if err := gitx.Move(ctx, root, rel, newRel); err != nil {
			return rel, nil, err
		}
	} else if err := mem.Rename(rel, newRel); err != nil {
		return rel, nil, err
	}
	if err := s.reindexPath(ctx, rel); err != nil {
		return rel, nil, err
	}
	if err := s.reindexPath(ctx, newRel); err != nil {
		return rel, nil, err
	}
	return newRel, &Fix{Path: rel, Kind: "bad-doc-name", Detail: "renamed to " + newRel}, nil
}

// conventionDocName derives the convention filename for an off-convention doc.
// Timestamped collections take a <first-commit-date>-0000-<slug>.md name (the
// 0000 time slot keeps the name self-describing when only a git date is known);
// adr files take a <next-number>-<slug>.md name. The slug derives from the doc
// title, falling back to the existing filename stem, then the doc type.
func (s *Service) conventionDocName(ctx context.Context, docType, rel string) (string, error) {
	slash := filepath.ToSlash(rel)
	cut := strings.LastIndex(slash, "/")
	dir, base := slash[:cut], slash[cut+1:]
	slug := slugify(s.docTitle(rel))
	if slug == "" {
		slug = slugify(strings.TrimSuffix(base, filepath.Ext(base)))
	}
	if slug == "" {
		slug = docType
	}
	if docType == "adr" {
		next, err := nextADRNumber(filepath.Join(s.Layout.Root, filepath.FromSlash(dir)))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%04d-%s.md", dir, next, slug), nil
	}
	return fmt.Sprintf("%s/%s-0000-%s.md", dir, s.docFirstDate(ctx, rel), slug), nil
}

// docTitle reads a doc's title frontmatter, returning an empty string when the
// note cannot be parsed or carries no title.
func (s *Service) docTitle(rel string) string {
	note, err := vault.Parse(s.Layout.Root, filepath.ToSlash(rel))
	if err != nil {
		return ""
	}
	return fmStringValue(note.Frontmatter, "title")
}

// rewriteFile replaces a file's contents in place through the memory store's
// delete-then-create idiom, keeping the write path-confined and serialized.
func rewriteFile(mem *memory.Store, rel, content string) error {
	if err := mem.Delete(rel); err != nil {
		return err
	}
	return mem.Create(rel, content)
}

// docFirstDate returns the doc's created date: its first git commit date when
// tracked, falling back to the file modification date for untracked or non-repo
// files.
func (s *Service) docFirstDate(ctx context.Context, rel string) string {
	if d, err := gitx.FirstCommitDate(ctx, s.Layout.Root, rel); err == nil && d != "" {
		return d
	}
	return fileDate(filepath.Join(s.Layout.Root, filepath.FromSlash(rel)))
}

// docLastDate returns the doc's updated date: its last git commit date when
// tracked, falling back to the file modification date for untracked or non-repo
// files.
func (s *Service) docLastDate(ctx context.Context, rel string) string {
	if d, err := gitx.LastCommitDate(ctx, s.Layout.Root, rel); err == nil && d != "" {
		return d
	}
	return fileDate(filepath.Join(s.Layout.Root, filepath.FromSlash(rel)))
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

func mergedSortedPaths(sets ...map[string]bool) []string {
	seen := map[string]bool{}
	var paths []string
	for _, m := range sets {
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
