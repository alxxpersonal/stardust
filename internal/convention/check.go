package convention

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alxxpersonal/stardust/internal/agentsync"
	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/config"
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
	allowedDocFolders, err := registeredDocFolders(root)
	if err != nil {
		return nil, err
	}
	docsActive := DocsConventionActive(root)
	cfg := checkConfig(root)
	sourceRoot, err := cfg.ResolveSourceRoot(root)
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
		if !docsActive {
			continue
		}
		if issue, ok := checkStrayDoc(rel, allowedDocFolders); ok {
			issues = append(issues, issue)
		}
		docType, ok := docTypeForPath(rel)
		if !ok {
			continue
		}
		issues = append(issues, checkDocFile(root, rel, docType, sourceRoot)...)
	}
	return issues, nil
}

func checkConfig(root string) config.Config {
	cfg, err := config.Load(config.Layout{Root: root}.Config())
	if err != nil {
		return config.Default()
	}
	return cfg
}

// registeredDocFolders returns the registered markdown collection folders under
// docs, using committed collection configs when present.
func registeredDocFolders(root string) ([]string, error) {
	cols, err := collections.Load(filepath.Join(root, config.DirName, "collections"))
	if err != nil {
		return nil, err
	}
	folders := make([]string, 0, len(cols))
	for _, col := range cols {
		folder := cleanFolder(col.Cfg.Path)
		if folder == "docs" || strings.HasPrefix(folder, "docs/") {
			folders = append(folders, folder)
		}
	}
	return folders, nil
}

// checkStrayDoc rejects markdown under docs that is outside registered folders.
func checkStrayDoc(rel string, allowedFolders []string) (ConventionIssue, bool) {
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "docs/") {
		return ConventionIssue{}, false
	}
	if rel == "docs/INDEX.md" || strings.HasPrefix(rel, "docs/templates/") {
		return ConventionIssue{}, false
	}
	for _, folder := range allowedFolders {
		if rel == folder || strings.HasPrefix(rel, folder+"/") {
			return ConventionIssue{}, false
		}
	}
	return ConventionIssue{
		Severity: "error",
		Kind:     "stray-doc",
		Path:     rel,
		Detail:   "markdown file under docs is outside registered collection folders",
	}, true
}

// cleanFolder normalizes a collection folder to slash-separated relative form.
func cleanFolder(path string) string {
	clean := filepath.ToSlash(filepath.Clean("/" + filepath.FromSlash(path)))
	return strings.Trim(clean, "/")
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

func checkDocFile(root, rel, docType, sourceRoot string) []ConventionIssue {
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
	if fields, ok := DocFields(root, docType); ok {
		issues = append(issues, checkDocFieldsSchema(rel, docType, fm, fields)...)
	}
	issues = append(issues, checkRelated(root, rel, fm)...)
	governsIssues, matched := checkGoverns(root, rel, fm, sourceRoot)
	issues = append(issues, governsIssues...)
	if fmString(fm, "status") == "Implemented" && len(matched) > 0 {
		issues = append(issues, checkStale(root, rel, matched)...)
	}
	issues = append(issues, checkDrift(root, rel, note)...)
	return issues
}

// DocFields returns the frontmatter schema the checker enforces for a doc of the
// given type. It prefers the committed .stardust/collections/<name>/config.toml
// when present, falling back to the built-in default for the collection, so an
// un-scaffolded repo still validates. ok is false for an unknown doc type.
func DocFields(root, docType string) ([]collections.Field, bool) {
	dc, ok := defaultDocCollectionFor(docType)
	if !ok {
		return nil, false
	}
	collectionsDir := filepath.Join(root, config.DirName, "collections")
	cfgPath := filepath.Join(collectionsDir, dc.Name, "config.toml")
	if _, err := os.Stat(cfgPath); err == nil {
		if col, err := collections.LoadOne(collectionsDir, dc.Name); err == nil {
			return col.Cfg.Fields, true
		}
	}
	return dc.Fields(), true
}

// defaultDocCollectionFor returns the built-in DocCollection for a singular doc
// type ("spec", "plan", "adr", "research").
func defaultDocCollectionFor(docType string) (DocCollection, bool) {
	for _, c := range DefaultDocCollections() {
		if c.Type == docType {
			return c, true
		}
	}
	return DocCollection{}, false
}

// checkDocFieldsSchema validates a doc's frontmatter against its collection
// schema via collections.Validate, one field at a time so every violation is
// reported. A missing required field is missing-doc-field; an invalid type or
// status keeps its dedicated kind; any other validation failure is bad-doc-field.
func checkDocFieldsSchema(rel, docType string, fm map[string]any, fields []collections.Field) []ConventionIssue {
	var issues []ConventionIssue
	for _, f := range fields {
		if v, present := fm[f.Name]; !present || v == nil {
			if f.Required {
				issues = append(issues, ConventionIssue{Severity: "error", Kind: "missing-doc-field", Path: rel, Detail: f.Name + " is required"})
			}
			continue
		}
		if err := collections.Validate(fm, []collections.Field{f}); err != nil {
			issues = append(issues, fieldViolation(rel, docType, f, fm, err))
		}
	}
	return issues
}

// fieldViolation maps a single-field validation failure to a ConventionIssue,
// preserving the dedicated type and status kinds and their detail wording.
func fieldViolation(rel, docType string, f collections.Field, fm map[string]any, err error) ConventionIssue {
	switch f.Name {
	case "type":
		return ConventionIssue{Severity: "error", Kind: "bad-doc-type", Path: rel, Detail: fmt.Sprintf("type %q does not match %q", fmString(fm, "type"), docType)}
	case "status":
		return ConventionIssue{Severity: "error", Kind: "bad-doc-status", Path: rel, Detail: fmt.Sprintf("status %q is not allowed for %s", fmString(fm, "status"), docType)}
	default:
		return ConventionIssue{Severity: "error", Kind: "bad-doc-field", Path: rel, Detail: strings.TrimPrefix(err.Error(), "validation: ")}
	}
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

func checkGoverns(root, rel string, fm map[string]any, sourceRoot string) ([]ConventionIssue, []string) {
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
			if sourceMatchesGoverns(sourceRoot, pattern) {
				continue
			}
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

func sourceMatchesGoverns(sourceRoot, pattern string) bool {
	if sourceRoot == "" {
		return false
	}
	files, err := filepath.Glob(filepath.Join(sourceRoot, filepath.FromSlash(pattern)))
	return err == nil && len(files) > 0
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

// checkDrift flags a doc whose referenced code has moved since the doc was last
// committed. The bindings are the doc-to-code references from ADR 0015 (related:
// targets and inline code-path spans that resolve to a non-markdown repo file),
// so unlike checkStale this is ungated by status: an ADR or research note that
// points at moved code is stale regardless of an Implemented marker it never
// carries. Each moved file yields one drift warning carrying its own commit
// count, phrased as a review prompt so it never reads as a hard error.
func checkDrift(root, rel string, note vault.Note) []ConventionIssue {
	refs := vault.CodeRefs(root, note)
	if len(refs) == 0 {
		return nil
	}
	ctx := context.Background()
	docCommit, err := gitx.LastCommit(ctx, root, rel)
	if err != nil || docCommit == "" {
		return nil
	}
	var issues []ConventionIssue
	for _, ref := range refs {
		count, err := gitx.CommitCountSince(ctx, root, docCommit, ref)
		if err != nil || count == 0 {
			continue
		}
		issues = append(issues, ConventionIssue{
			Severity: "warn",
			Kind:     "drift",
			Path:     rel,
			Detail:   fmt.Sprintf("references `%s`, which moved %s since this doc was last touched; review", ref, commitNoun(count)),
		})
	}
	return issues
}

// commitNoun renders a commit count with a singular or plural noun, e.g.
// "1 commit" or "3 commits", for drift review prompts.
func commitNoun(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
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
