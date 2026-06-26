package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// --- Fingerprint ---

// fingerprintDigits collapses integer runs in a detail so a varying count (a
// drift commit distance, a duplicate-file tally) does not churn an issue's
// identity across runs.
var fingerprintDigits = regexp.MustCompile(`\d+`)

// Fingerprint returns the stable identity of a check issue for the CI ratchet:
// the tuple of kind, path, and a normalized detail (whitespace collapsed, digit
// runs replaced) hashed to a hex string. Two issues share a fingerprint exactly
// when they are the same problem, so a fixed-then-reintroduced issue re-fires.
func Fingerprint(issue Issue) string {
	detail := strings.Join(strings.Fields(issue.Detail), " ")
	detail = fingerprintDigits.ReplaceAllString(detail, "N")
	sum := sha256.Sum256([]byte(issue.Kind + "\x00" + issue.Path + "\x00" + detail))
	return hex.EncodeToString(sum[:])
}

// --- Baseline ---

// BaselineIssue is one committed known-issue record. It stores the human-legible
// tuple (kind, path, detail) so the baseline diff is reviewable; matching is done
// through Fingerprint, not raw equality.
type BaselineIssue struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

// Baseline is the committed set of known-issue fingerprints the CI ratchet
// subtracts from the current issue set, so only new issues fail the gate.
type Baseline struct {
	Issues []BaselineIssue `json:"issues"`
}

// fingerprints returns the set of fingerprints the baseline covers.
func (b Baseline) fingerprints() map[string]bool {
	out := make(map[string]bool, len(b.Issues))
	for _, bi := range b.Issues {
		out[Fingerprint(Issue{Kind: bi.Kind, Path: bi.Path, Detail: bi.Detail})] = true
	}
	return out
}

// LoadBaseline reads the committed baseline at path, returning an empty baseline
// when the file does not exist so an un-adopted repo has no backlog to subtract.
func LoadBaseline(path string) (Baseline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Baseline{}, nil
		}
		return Baseline{}, fmt.Errorf("read baseline %s: %w", path, err)
	}
	var b Baseline
	if err := json.Unmarshal(raw, &b); err != nil {
		return Baseline{}, fmt.Errorf("parse baseline %s: %w", path, err)
	}
	return b, nil
}

// Save writes the baseline to path as indented JSON, creating parent dirs.
func (b Baseline) Save(path string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create baseline dir: %w", err)
		}
	}
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write baseline %s: %w", path, err)
	}
	return nil
}

// --- CI ratchet ---

// CIResult is the outcome of a baseline-ratcheted check: the issues new since the
// baseline was snapshotted, plus counts for context. NewErrors drives the exit
// code, so a backlog of known issues adopts the gate green.
type CIResult struct {
	New       []Issue `json:"new"`
	NewErrors int     `json:"new_errors"`
	Total     int     `json:"total"`
	Baseline  int     `json:"baseline"`
	Markdown  string  `json:"markdown"`
}

// CheckCI runs Check, subtracts the committed baseline by fingerprint, and
// returns only the issues new since the baseline was snapshotted. NewErrors
// counts the error-severity newcomers; warnings (drift, orphans) are reported but
// do not by themselves fail the gate.
func (s *Service) CheckCI(ctx context.Context) (CIResult, error) {
	res, err := s.Check(ctx)
	if err != nil {
		return CIResult{}, err
	}
	base, err := LoadBaseline(s.Layout.Baseline())
	if err != nil {
		return CIResult{}, err
	}
	known := base.fingerprints()

	var fresh []Issue
	newErrors := 0
	for _, is := range res.Issues {
		if known[Fingerprint(is)] {
			continue
		}
		fresh = append(fresh, is)
		if is.Severity == "error" {
			newErrors++
		}
	}
	out := CIResult{New: fresh, NewErrors: newErrors, Total: len(res.Issues), Baseline: len(base.Issues)}
	out.Markdown = renderCI(out)
	return out, nil
}

// UpdateBaseline snapshots the current issue set to .stardust/baseline.json so the
// ratchet treats every present issue as known, then returns the written baseline.
func (s *Service) UpdateBaseline(ctx context.Context) (Baseline, error) {
	res, err := s.Check(ctx)
	if err != nil {
		return Baseline{}, err
	}
	b := Baseline{Issues: make([]BaselineIssue, 0, len(res.Issues))}
	for _, is := range res.Issues {
		b.Issues = append(b.Issues, BaselineIssue{Kind: is.Kind, Path: is.Path, Detail: is.Detail})
	}
	if err := b.Save(s.Layout.Baseline()); err != nil {
		return Baseline{}, err
	}
	return b, nil
}

// renderCI renders the CI ratchet report: a header with the new/baseline/total
// counts, then one line per new issue, mirroring renderCheck's format.
func renderCI(res CIResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Vault check (CI)\n\n%d new issue(s) since baseline (%d baselined, %d total).\n\n", len(res.New), res.Baseline, res.Total)
	if len(res.New) == 0 {
		b.WriteString("No new issues. Baseline holds.\n")
		return b.String()
	}
	for _, is := range res.New {
		label := "warn"
		if is.Severity == "error" {
			label = "ERROR"
		}
		fmt.Fprintf(&b, "- **%s** [%s] `%s` - %s\n", label, is.Kind, is.Path, is.Detail)
	}
	return b.String()
}
