// Package convention centralizes Stardust docs and agent metadata rules.
package convention

import (
	"fmt"
	"strings"

	"github.com/alxxpersonal/stardust/internal/collections"
)

// DocType identifies a Stardust convention document type.
type DocType string

// Supported convention document types.
const (
	DocTypeSpec     DocType = "spec"
	DocTypePlan     DocType = "plan"
	DocTypeADR      DocType = "adr"
	DocTypeResearch DocType = "research"
)

// DocCollection describes one docs collection scaffold. Type is the singular
// frontmatter type its records carry ("spec", "plan", "adr", "research") and is
// the lone member of the type field's enum.
type DocCollection struct {
	Name        string
	Type        string
	Path        string
	Description string
	Statuses    []string
}

// DefaultDocCollections returns the standard specs, plans, adr, and research collections.
func DefaultDocCollections() []DocCollection {
	return []DocCollection{
		{Name: "specs", Type: "spec", Path: "docs/specs", Description: "technical specs", Statuses: []string{"Draft", "In Review", "Approved", "Implemented", "Superseded"}},
		{Name: "plans", Type: "plan", Path: "docs/plans", Description: "implementation plans", Statuses: []string{"Draft", "Active", "Done", "Abandoned"}},
		{Name: "adr", Type: "adr", Path: "docs/adr", Description: "architecture decision records", Statuses: []string{"Proposed", "Accepted", "Deferred", "Rejected", "Superseded"}},
		{Name: "research", Type: "research", Path: "docs/research", Description: "research notes", Statuses: []string{"Active", "Archived", "Superseded"}},
	}
}

// Fields returns the ordered, full frontmatter schema for the collection: the
// required title, type, status, created, and updated columns plus the optional
// governs and related reference columns. It is the single declarative source the
// scaffolder codegens from, the checker validates against, and the autofixer
// derives fixability from, so a generated doc cannot fail its own linter.
func (c DocCollection) Fields() []collections.Field {
	return []collections.Field{
		{Name: "title", Type: collections.TypeString, Required: true},
		{Name: "type", Type: collections.TypeEnum, Required: true, Enum: []string{c.Type}},
		{Name: "status", Type: collections.TypeEnum, Required: true, Enum: append([]string(nil), c.Statuses...)},
		{Name: "created", Type: collections.TypeDate, Required: true},
		{Name: "updated", Type: collections.TypeDate, Required: true},
		{Name: "governs", Type: collections.TypeRef, Required: false},
		{Name: "related", Type: collections.TypeRef, Required: false},
	}
}

// DocStatusAllowed reports whether status is valid for docType.
func DocStatusAllowed(docType, status string) bool {
	for _, allowed := range statusesFor(DocType(docType)) {
		if status == allowed {
			return true
		}
	}
	return false
}

// StringList reads a frontmatter field as a list of strings.
func StringList(frontmatter map[string]any, key string) ([]string, error) {
	raw, ok := frontmatter[key]
	if !ok || raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case []string:
		out := make([]string, len(v))
		copy(out, v)
		return out, nil
	case []any:
		out := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%d] must be a string", key, i)
			}
			out = append(out, strings.TrimSpace(s))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be a list of strings", key)
	}
}

func statusesFor(docType DocType) []string {
	for _, c := range DefaultDocCollections() {
		switch docType {
		case DocTypeSpec:
			if c.Name == "specs" {
				return c.Statuses
			}
		case DocTypePlan:
			if c.Name == "plans" {
				return c.Statuses
			}
		case DocTypeADR:
			if c.Name == "adr" {
				return c.Statuses
			}
		case DocTypeResearch:
			if c.Name == "research" {
				return c.Statuses
			}
		}
	}
	return nil
}
