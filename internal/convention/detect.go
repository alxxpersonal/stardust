package convention

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- Directory kind detection ---

// Kind is the detected nature of a directory: a code repo that defaults to the
// docs convention, or a plain markdown vault that does not.
type Kind int

// The directory kinds DetectKind can return.
const (
	// KindPlainVault is a human markdown vault that does not want the docs convention.
	KindPlainVault Kind = iota
	// KindCodeRepo is a code repository that defaults to the docs convention.
	KindCodeRepo
)

// WantsDocs reports whether init should scaffold the docs convention by default
// for this kind.
func (k Kind) WantsDocs() bool { return k == KindCodeRepo }

// Label returns the stable status string for the kind.
func (k Kind) Label() string {
	if k == KindCodeRepo {
		return "code-repo-with-docs"
	}
	return "plain-vault"
}

// Describe returns the one-line init detection sentence, including the override
// flag a caller would use to flip the decision.
func (k Kind) Describe() string {
	if k == KindCodeRepo {
		return "detected a code repo, scaffolding the docs convention (use --no-docs to skip)"
	}
	return "detected a plain vault, skipping the docs convention (use --docs to scaffold)"
}

// DocsConventionActive reports whether root should enforce the Stardust docs
// convention. Committed docs collection configs opt in explicitly; otherwise it
// follows the same code-repo detection used by init.
func DocsConventionActive(root string) bool {
	folders, err := registeredDocFolders(root)
	if err == nil && len(folders) > 0 {
		return true
	}
	kind, err := DetectKind(root)
	return err == nil && kind.WantsDocs()
}

// sourceManifests are the top-level manifest files that mark a code repo.
var sourceManifests = map[string]bool{
	"go.mod":       true,
	"package.json": true,
	"Cargo.toml":   true,
}

// sourceExts are the source-file extensions that mark a code repo.
var sourceExts = map[string]bool{
	".go": true,
	".ts": true,
	".py": true,
	".rs": true,
}

// DetectKind sniffs the top level of dir (non-recursive) and classifies it as a
// code repo or a plain vault. Precedence, first match wins:
//
//  1. an .obsidian directory present -> KindPlainVault
//  2. a source marker present        -> KindCodeRepo
//  3. markdown-dominant              -> KindPlainVault
//  4. a .git directory present       -> KindCodeRepo
//  5. otherwise                      -> KindPlainVault
//
// A source marker is a go.mod, package.json, or Cargo.toml manifest, or any
// top-level .go, .ts, .py, or .rs file. Markdown-dominant means at least one .md
// file with the .md count greater than or equal to the count of non-markdown
// regular files, dotfiles excluded from both sides. The .git rule sits below
// markdown-dominance on purpose: a Stardust vault is itself git-backed, so .git
// alone must not reclassify a human markdown vault. An unreadable dir yields
// KindPlainVault and a wrapped error so callers can default safely.
func DetectKind(dir string) (Kind, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return KindPlainVault, fmt.Errorf("detect kind in %s: %w", dir, err)
	}

	var hasObsidian, hasGit, hasSourceMarker bool
	var mdCount, nonMdCount int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			switch name {
			case ".obsidian":
				hasObsidian = true
			case ".git":
				hasGit = true
			}
			continue
		}
		if !e.Type().IsRegular() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if sourceManifests[name] || sourceExts[ext] {
			hasSourceMarker = true
		}
		// dotfiles are excluded from the markdown-dominance tally on both sides.
		if strings.HasPrefix(name, ".") {
			continue
		}
		if ext == ".md" {
			mdCount++
		} else {
			nonMdCount++
		}
	}

	switch {
	case hasObsidian:
		return KindPlainVault, nil
	case hasSourceMarker:
		return KindCodeRepo, nil
	case mdCount > 0 && mdCount >= nonMdCount:
		return KindPlainVault, nil
	case hasGit:
		return KindCodeRepo, nil
	default:
		return KindPlainVault, nil
	}
}
