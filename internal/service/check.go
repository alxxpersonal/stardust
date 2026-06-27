package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/alxxpersonal/stardust/internal/convention"
	"github.com/alxxpersonal/stardust/internal/graph"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// Issue is one vault-integrity problem.
type Issue struct {
	Severity string `json:"severity"` // error | warn
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Detail   string `json:"detail"`
}

// CheckResult is the outcome of a vault integrity check.
type CheckResult struct {
	Issues   []Issue `json:"issues"`
	Errors   int     `json:"errors"`
	Warnings int     `json:"warnings"`
	Markdown string  `json:"markdown"`
}

// Check validates the vault: broken wikilinks and malformed frontmatter are
// errors; orphan notes, missing titles, and duplicate note names are warnings.
// It derives most of this from the link graph, so it is cheap.
func (s *Service) Check(_ context.Context) (CheckResult, error) {
	var issues []Issue

	g, err := graph.Build(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return CheckResult{}, err
	}
	for _, bl := range g.BrokenLinks() {
		issues = append(issues, Issue{Severity: "error", Kind: "broken-link", Path: bl.From, Detail: "[[" + bl.Target + "]] resolves to no note"})
	}
	for _, p := range g.Orphans() {
		issues = append(issues, Issue{Severity: "warn", Kind: "orphan", Path: p, Detail: "no links in or out"})
	}

	paths, err := vault.Scan(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return CheckResult{}, err
	}
	nameToPaths := map[string][]string{}
	requireExplicitTitle := convention.DocsConventionActive(s.Layout.Root)
	for _, rel := range paths {
		probs, err := vault.CheckFileWithOptions(s.Layout.Root, rel, vault.CheckOptions{
			RequireExplicitTitle: requireExplicitTitle,
		})
		if err != nil {
			continue
		}
		for _, pr := range probs {
			sev := "warn"
			if pr.Kind == "bad-frontmatter" {
				sev = "error"
			}
			issues = append(issues, Issue{Severity: sev, Kind: pr.Kind, Path: rel, Detail: pr.Detail})
		}
		key := vault.CollectionKey(rel)
		nameToPaths[key] = append(nameToPaths[key], rel)
	}
	for name, ps := range nameToPaths {
		if len(ps) > 1 {
			sort.Strings(ps)
			issues = append(issues, Issue{
				Severity: "warn",
				Kind:     "duplicate-name",
				Path:     ps[0],
				Detail:   fmt.Sprintf("note name %q is shared by %d files (%s); wikilinks to it are ambiguous", name, len(ps), strings.Join(ps, ", ")),
			})
		}
	}

	docIssues, err := convention.CheckDocs(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return CheckResult{}, err
	}
	issues = append(issues, mapConventionIssues(docIssues)...)
	skillIssues, err := convention.CheckSkills(s.Layout.Root)
	if err != nil {
		return CheckResult{}, err
	}
	issues = append(issues, mapConventionIssues(skillIssues)...)

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Severity != issues[j].Severity {
			return issues[i].Severity == "error"
		}
		return issues[i].Path < issues[j].Path
	})

	res := CheckResult{Issues: issues}
	for _, is := range issues {
		if is.Severity == "error" {
			res.Errors++
		} else {
			res.Warnings++
		}
	}
	res.Markdown = renderCheck(res)
	return res, nil
}

func mapConventionIssues(in []convention.ConventionIssue) []Issue {
	out := make([]Issue, 0, len(in))
	for _, issue := range in {
		out = append(out, Issue{Severity: issue.Severity, Kind: issue.Kind, Path: issue.Path, Detail: issue.Detail})
	}
	return out
}

func renderCheck(res CheckResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Vault check\n\n%d errors, %d warnings.\n\n", res.Errors, res.Warnings)
	if len(res.Issues) == 0 {
		b.WriteString("Clean. No issues found.\n")
		return b.String()
	}
	for _, is := range res.Issues {
		label := "warn"
		if is.Severity == "error" {
			label = "ERROR"
		}
		fmt.Fprintf(&b, "- **%s** [%s] `%s` - %s\n", label, is.Kind, is.Path, is.Detail)
	}
	return b.String()
}
