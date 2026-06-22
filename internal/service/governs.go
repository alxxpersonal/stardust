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
