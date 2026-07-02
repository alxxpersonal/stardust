package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/convention"
	"github.com/alxxpersonal/stardust/internal/doclinks"
	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/graph"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// GovernedDoc is a document whose governs patterns match a code path.
type GovernedDoc struct {
	DocPath string   `json:"doc_path"`
	Title   string   `json:"title"`
	Type    string   `json:"type"`
	Status  string   `json:"status"`
	Governs []string `json:"governs"`
	Matched []string `json:"matched"`

	DocCommit      string `json:"doc_commit"`
	LastCodeCommit string `json:"last_code_commit"`
	ChangedCommits int    `json:"changed_commits"`
	Stale          bool   `json:"stale"`
}

// GoverningResult contains governing docs for one repo-relative path.
type GoverningResult struct {
	Path     string        `json:"path"`
	Docs     []GovernedDoc `json:"docs"`
	Markdown string        `json:"markdown"`
}

// StaleResult is the repo-wide set of stale governed docs: docs whose governed
// code changed after the doc's last commit. It is the inverse of GoverningDocs.
type StaleResult struct {
	Docs     []GovernedDoc `json:"docs"`
	Markdown string        `json:"markdown"`
}

// stalePathSentinel is a repo path that can never exist, passed to
// MatchGovernedPath so it returns every file a governs glob matches rather than
// short-circuiting on an exact-path match.
const stalePathSentinel = "\x00"

// StaleDocs lists every Implemented doc whose governed code changed after the
// doc's last commit, across the specs, plans, adr, and research collections. It
// reuses the same staleness math as GoverningDocs with the loop inverted: it
// walks docs (not a path) and keeps only those that annotateStaleness marks
// stale.
func (s *Service) StaleDocs(ctx context.Context) (StaleResult, error) {
	var docs []GovernedDoc
	for _, collection := range []string{"specs", "plans", "adr", "research"} {
		if _, err := collections.LoadOne(s.Layout.Collections(), collection); err != nil {
			continue
		}
		list, err := s.ListRecords(ctx, collection, nil, "path", 0, 0)
		if err != nil {
			return StaleResult{}, err
		}
		for _, record := range list.Records {
			doc, ok, err := s.staleDoc(ctx, collection, record)
			if err != nil {
				return StaleResult{}, err
			}
			if ok && doc.Stale {
				docs = append(docs, doc)
			}
		}
	}
	sort.SliceStable(docs, func(i, j int) bool {
		if typeRank(docs[i].Type) != typeRank(docs[j].Type) {
			return typeRank(docs[i].Type) < typeRank(docs[j].Type)
		}
		return docs[i].DocPath < docs[j].DocPath
	})
	result := StaleResult{Docs: docs}
	result.Markdown = renderStaleMarkdown(result)
	return result, nil
}

// staleDoc builds a GovernedDoc for record carrying every file its governs
// globs match, then annotates staleness. ok is false when the doc declares no
// governs patterns or none of them match any file.
func (s *Service) staleDoc(ctx context.Context, collection string, record Record) (GovernedDoc, bool, error) {
	governs, err := convention.StringList(record.Frontmatter, "governs")
	if err != nil {
		return GovernedDoc{}, false, fmt.Errorf("doc %s: %w", record.Path, err)
	}
	if len(governs) == 0 {
		return GovernedDoc{}, false, nil
	}
	var matched []string
	for _, pattern := range governs {
		_, files, err := doclinks.MatchGovernedPath(s.Layout.Root, pattern, stalePathSentinel)
		if err != nil {
			return GovernedDoc{}, false, err
		}
		matched = append(matched, files...)
	}
	if len(matched) == 0 {
		return GovernedDoc{}, false, nil
	}
	doc := GovernedDoc{
		DocPath: record.Path,
		Title:   record.Title,
		Type:    docTypeForCollection(collection),
		Status:  frontmatterString(record.Frontmatter, "status"),
		Governs: governs,
		Matched: matched,
	}
	if err := s.annotateStaleness(ctx, &doc); err != nil {
		return GovernedDoc{}, false, err
	}
	return doc, true, nil
}

// GoverningDocs returns convention docs whose governs patterns match path.
func (s *Service) GoverningDocs(ctx context.Context, path string) (GoverningResult, error) {
	clean, err := cleanRel(path)
	if err != nil {
		return GoverningResult{}, err
	}
	var docs []GovernedDoc
	for _, collection := range []string{"specs", "plans", "adr", "research"} {
		if _, err := collections.LoadOne(s.Layout.Collections(), collection); err != nil {
			continue
		}
		list, err := s.ListRecords(ctx, collection, nil, "path", 0, 0)
		if err != nil {
			return GoverningResult{}, err
		}
		for _, record := range list.Records {
			doc, ok, err := s.governingDoc(ctx, collection, record, clean)
			if err != nil {
				return GoverningResult{}, err
			}
			if ok {
				docs = append(docs, doc)
			}
		}
	}
	sort.SliceStable(docs, func(i, j int) bool {
		if typeRank(docs[i].Type) != typeRank(docs[j].Type) {
			return typeRank(docs[i].Type) < typeRank(docs[j].Type)
		}
		return docs[i].DocPath < docs[j].DocPath
	})
	result := GoverningResult{Path: clean, Docs: docs}
	result.Markdown = renderGoverningMarkdown(result)
	return result, nil
}

func (s *Service) governingDoc(ctx context.Context, collection string, record Record, path string) (GovernedDoc, bool, error) {
	governs, err := convention.StringList(record.Frontmatter, "governs")
	if err != nil {
		return GovernedDoc{}, false, fmt.Errorf("doc %s: %w", record.Path, err)
	}
	if len(governs) == 0 {
		return GovernedDoc{}, false, nil
	}
	var matched []string
	for _, pattern := range governs {
		ok, files, err := doclinks.MatchGovernedPath(s.Layout.Root, pattern, path)
		if err != nil {
			return GovernedDoc{}, false, err
		}
		if ok {
			matched = append(matched, files...)
		}
	}
	if len(matched) == 0 {
		return GovernedDoc{}, false, nil
	}
	doc := GovernedDoc{
		DocPath: record.Path,
		Title:   record.Title,
		Type:    docTypeForCollection(collection),
		Status:  frontmatterString(record.Frontmatter, "status"),
		Governs: governs,
		Matched: matched,
	}
	if err := s.annotateStaleness(ctx, &doc); err != nil {
		return GovernedDoc{}, false, err
	}
	return doc, true, nil
}

// --- Reference-bound drift (ungated) ---

// DriftBinding is one moved code file a doc references, carrying the number of
// commits to that file since the doc's last commit.
type DriftBinding struct {
	File           string `json:"file"`
	ChangedCommits int    `json:"changed_commits"`
	Source         string `json:"source,omitempty"`
}

const driftSourceRepo = "source_repo"

type driftRef struct {
	file   string
	root   string
	source string
}

// DriftDoc is a doc bound to code through a related: or inline-path reference
// (ADR 0015) whose bound code moved since the doc's last commit. Unlike a stale
// governed doc it is ungated by status: a doc that points at moved code drifts
// regardless of an Implemented marker it never carries.
type DriftDoc struct {
	DocPath  string         `json:"doc_path"`
	Title    string         `json:"title"`
	Type     string         `json:"type"`
	Bindings []DriftBinding `json:"bindings"`
}

// DriftResult is the repo-wide set of docs referencing moved code.
type DriftResult struct {
	Docs     []DriftDoc `json:"docs"`
	Markdown string     `json:"markdown"`
}

// DriftDocs reports every doc whose code references moved since the doc's last
// commit. It binds each doc to code through the reference edges of ADR 0015
// (related: targets and inline code paths resolving to a non-markdown repo file,
// captured as the link graph's CodeRefs) plus governs: frontmatter, then measures
// commit-distance per bound file. It is the ungated, reference-bound counterpart
// to StaleDocs; the governs:-plus-Implemented path (StaleDocs) is unchanged.
func (s *Service) DriftDocs(ctx context.Context) (DriftResult, error) {
	g, err := graph.Build(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return DriftResult{}, err
	}
	var docs []DriftDoc
	fallbackType := s.fallbackDriftDocType()
	for _, node := range g.Nodes {
		docType, ok := convention.DocTypeForPath(node.Path)
		refs := localDriftRefs(codeRefTargets(node.CodeRefs))
		governedRefs, err := s.governedDriftRefs(node.Path)
		if err != nil {
			return DriftResult{}, err
		}
		refs = appendUniqueDriftRefs(refs, governedRefs...)
		if !ok {
			if len(governedRefs) == 0 {
				continue
			}
			docType = fallbackType
		}
		if len(refs) == 0 {
			continue
		}
		bindings, err := s.docDrift(ctx, node.Path, refs)
		if err != nil {
			return DriftResult{}, err
		}
		if len(bindings) == 0 {
			continue
		}
		docs = append(docs, DriftDoc{DocPath: node.Path, Title: node.Title, Type: docType, Bindings: bindings})
	}
	sort.SliceStable(docs, func(i, j int) bool {
		if typeRank(docs[i].Type) != typeRank(docs[j].Type) {
			return typeRank(docs[i].Type) < typeRank(docs[j].Type)
		}
		return docs[i].DocPath < docs[j].DocPath
	})
	result := DriftResult{Docs: docs}
	result.Markdown = renderDriftMarkdown(result)
	return result, nil
}

func (s *Service) fallbackDriftDocType() string {
	kind, err := convention.DetectKind(s.Layout.Root)
	if err == nil && kind == convention.KindGitHubWiki {
		return "wiki"
	}
	return "vault"
}

func (s *Service) governedDriftRefs(path string) ([]driftRef, error) {
	note, err := vault.Parse(s.Layout.Root, path)
	if err != nil {
		return nil, err
	}
	governs, err := convention.StringList(note.Frontmatter, "governs")
	if err != nil {
		return nil, fmt.Errorf("doc %s: %w", path, err)
	}
	var refs []driftRef
	for _, pattern := range governs {
		matches, err := s.matchGovernedDriftRefs(pattern)
		if err != nil {
			return nil, err
		}
		refs = appendUniqueDriftRefs(refs, matches...)
	}
	return refs, nil
}

func (s *Service) matchGovernedDriftRefs(pattern string) ([]driftRef, error) {
	_, files, err := doclinks.MatchGovernedPath(s.Layout.Root, pattern, stalePathSentinel)
	if err != nil {
		return nil, err
	}
	if len(files) > 0 {
		return localDriftRefs(files), nil
	}
	sourceRoot, _, err := convention.ResolveSourceRoot(s.Config, s.Layout.Root)
	if err != nil {
		return nil, err
	}
	if sourceRoot == "" {
		return nil, nil
	}
	_, files, err = doclinks.MatchGovernedPath(sourceRoot, pattern, stalePathSentinel)
	if err != nil {
		return nil, err
	}
	refs := make([]driftRef, 0, len(files))
	for _, file := range files {
		refs = append(refs, driftRef{file: file, root: sourceRoot, source: driftSourceRepo})
	}
	return refs, nil
}

// docDrift returns the moved code bindings for a doc: for each referenced code
// file, the number of commits to it since the doc's last commit, keeping only
// files that moved. It returns nil outside a git repo, when the doc is untracked,
// or when no referenced file moved, so a non-repo or fresh doc never drifts.
func (s *Service) docDrift(ctx context.Context, docPath string, codeRefs []driftRef) ([]DriftBinding, error) {
	if len(codeRefs) == 0 {
		return nil, nil
	}
	docCommit, err := gitx.LastCommit(ctx, s.Layout.Root, docPath)
	if err != nil {
		return nil, fmt.Errorf("last doc commit %s: %w", docPath, err)
	}
	if docCommit == "" {
		return nil, nil
	}
	var docUnix int64
	if hasSourceRepoRef(codeRefs) {
		docUnix, err = gitx.LastCommitUnix(ctx, s.Layout.Root, docPath)
		if err != nil {
			return nil, fmt.Errorf("last doc commit time %s: %w", docPath, err)
		}
		if docUnix == 0 {
			return nil, nil
		}
	}
	var out []DriftBinding
	for _, ref := range codeRefs {
		count, err := s.driftCount(ctx, docCommit, docUnix, ref)
		if err != nil {
			return nil, fmt.Errorf("drift count for %s referencing %s: %w", docPath, ref.file, err)
		}
		if count > 0 {
			out = append(out, DriftBinding{File: ref.file, ChangedCommits: count, Source: ref.source})
		}
	}
	return out, nil
}

func (s *Service) driftCount(ctx context.Context, docCommit string, docUnix int64, ref driftRef) (int, error) {
	if ref.source == driftSourceRepo {
		return gitx.CommitCountSinceUnix(ctx, ref.root, docUnix, ref.file)
	}
	return gitx.CommitCountSince(ctx, s.Layout.Root, docCommit, ref.file)
}

func hasSourceRepoRef(refs []driftRef) bool {
	for _, ref := range refs {
		if ref.source == driftSourceRepo {
			return true
		}
	}
	return false
}

// codeRefTargets returns the unique target paths of a node's code references.
func codeRefTargets(edges []graph.Edge) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range edges {
		if e.Target == "" || seen[e.Target] {
			continue
		}
		seen[e.Target] = true
		out = append(out, e.Target)
	}
	return out
}

func localDriftRefs(paths []string) []driftRef {
	refs := make([]driftRef, 0, len(paths))
	for _, path := range paths {
		refs = append(refs, driftRef{file: path})
	}
	return refs
}

func appendUniqueDriftRefs(items []driftRef, more ...driftRef) []driftRef {
	seen := make(map[string]bool, len(items)+len(more))
	out := make([]driftRef, 0, len(items)+len(more))
	for _, item := range items {
		if item.file == "" {
			continue
		}
		key := driftRefKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	for _, item := range more {
		if item.file == "" {
			continue
		}
		key := driftRefKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func driftRefKey(ref driftRef) string {
	return ref.source + "\x00" + ref.root + "\x00" + ref.file
}

func driftBindingLabel(bind DriftBinding) string {
	if bind.Source == driftSourceRepo {
		return bind.File + " (source repo)"
	}
	return bind.File
}

// renderDriftMarkdown renders the drifted-docs report, one row per moved binding.
func renderDriftMarkdown(result DriftResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Drifted Docs\n\n")
	if len(result.Docs) == 0 {
		fmt.Fprintln(&b, "No drifted docs found.")
		return b.String()
	}
	fmt.Fprintln(&b, "| Type | Title | Doc | Referenced | Commits |")
	fmt.Fprintln(&b, "|---|---|---|---|---|")
	for _, doc := range result.Docs {
		for _, bind := range doc.Bindings {
			fmt.Fprintf(&b, "| %s | %s | `%s` | `%s` | %d |\n", doc.Type, doc.Title, doc.DocPath, driftBindingLabel(bind), bind.ChangedCommits)
		}
	}
	return b.String()
}

func (s *Service) annotateStaleness(ctx context.Context, doc *GovernedDoc) error {
	if doc.Status != "Implemented" {
		return nil
	}
	docCommit, err := gitx.LastCommit(ctx, s.Layout.Root, doc.DocPath)
	if err != nil {
		return fmt.Errorf("last doc commit %s: %w", doc.DocPath, err)
	}
	doc.DocCommit = docCommit
	lastCodeCommit, err := gitx.LastCommit(ctx, s.Layout.Root, doc.Matched...)
	if err != nil {
		return fmt.Errorf("last code commit for %s: %w", doc.DocPath, err)
	}
	doc.LastCodeCommit = lastCodeCommit
	changed, err := gitx.CommitCountSince(ctx, s.Layout.Root, docCommit, doc.Matched...)
	if err != nil {
		return fmt.Errorf("changed commits for %s: %w", doc.DocPath, err)
	}
	doc.ChangedCommits = changed
	doc.Stale = changed > 0
	return nil
}

func renderGoverningMarkdown(result GoverningResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Governing Docs\n\n")
	if len(result.Docs) == 0 {
		fmt.Fprintf(&b, "No governing docs found for `%s`.\n", result.Path)
		return b.String()
	}
	fmt.Fprintf(&b, "Path: `%s`\n\n", result.Path)
	fmt.Fprintln(&b, "| Type | Title | Status | Doc | Matched | Stale |")
	fmt.Fprintln(&b, "|---|---|---|---|---|---|")
	for _, doc := range result.Docs {
		stale := ""
		if doc.Stale {
			stale = "yes"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | `%s` | `%s` | %s |\n", doc.Type, doc.Title, doc.Status, doc.DocPath, strings.Join(doc.Matched, ", "), stale)
	}
	return b.String()
}

func renderStaleMarkdown(result StaleResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Stale Docs\n\n")
	if len(result.Docs) == 0 {
		fmt.Fprintln(&b, "No stale docs found.")
		return b.String()
	}
	fmt.Fprintln(&b, "| Type | Title | Status | Doc | Commits | Matched |")
	fmt.Fprintln(&b, "|---|---|---|---|---|---|")
	for _, doc := range result.Docs {
		fmt.Fprintf(&b, "| %s | %s | %s | `%s` | %d | `%s` |\n", doc.Type, doc.Title, doc.Status, doc.DocPath, doc.ChangedCommits, strings.Join(doc.Matched, ", "))
	}
	return b.String()
}

func docTypeForCollection(collection string) string {
	switch collection {
	case "specs":
		return "spec"
	case "plans":
		return "plan"
	default:
		return collection
	}
}

func typeRank(docType string) int {
	switch docType {
	case "spec":
		return 0
	case "plan":
		return 1
	case "adr":
		return 2
	case "research":
		return 3
	default:
		return 4
	}
}
