package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/convention"
	"github.com/alxxpersonal/stardust/internal/doclinks"
)

// GovernedDoc is a document whose governs patterns match a code path.
type GovernedDoc struct {
	DocPath string   `json:"doc_path"`
	Title   string   `json:"title"`
	Type    string   `json:"type"`
	Status  string   `json:"status"`
	Governs []string `json:"governs"`
	Matched []string `json:"matched"`
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
			doc, ok, err := s.governingDoc(collection, record, clean)
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

func (s *Service) governingDoc(collection string, record Record, path string) (GovernedDoc, bool, error) {
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
	return GovernedDoc{
		DocPath: record.Path,
		Title:   record.Title,
		Type:    docTypeForCollection(collection),
		Status:  frontmatterString(record.Frontmatter, "status"),
		Governs: governs,
		Matched: matched,
	}, true, nil
}

func renderGoverningMarkdown(result GoverningResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Governing Docs\n\n")
	if len(result.Docs) == 0 {
		fmt.Fprintf(&b, "No governing docs found for `%s`.\n", result.Path)
		return b.String()
	}
	fmt.Fprintf(&b, "Path: `%s`\n\n", result.Path)
	fmt.Fprintln(&b, "| Type | Title | Status | Doc | Matched |")
	fmt.Fprintln(&b, "|---|---|---|---|---|")
	for _, doc := range result.Docs {
		fmt.Fprintf(&b, "| %s | %s | %s | `%s` | `%s` |\n", doc.Type, doc.Title, doc.Status, doc.DocPath, strings.Join(doc.Matched, ", "))
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
