package convention

import (
	"bufio"
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
	// KindGitHubWiki is a GitHub wiki clone or a markdown repo shaped like one.
	KindGitHubWiki
	// KindCodeRepo is a code repository that defaults to the docs convention.
	KindCodeRepo
)

// WantsDocs reports whether init should scaffold the docs convention by default
// for this kind.
func (k Kind) WantsDocs() bool { return k == KindCodeRepo }

// Label returns the stable status string for the kind.
func (k Kind) Label() string {
	switch k {
	case KindCodeRepo:
		return "code-repo-with-docs"
	case KindGitHubWiki:
		return "github-wiki"
	default:
		return "plain-vault"
	}
}

// Describe returns the one-line init detection sentence, including the override
// flag a caller would use to flip the decision.
func (k Kind) Describe() string {
	switch k {
	case KindCodeRepo:
		return "detected a code repo, scaffolding the docs convention (use --no-docs to skip)"
	case KindGitHubWiki:
		return "detected a github wiki, skipping the docs convention (use --docs to scaffold)"
	default:
		return "detected a plain vault, skipping the docs convention (use --docs to scaffold)"
	}
}

// DocsConventionActive reports whether root should enforce the Stardust docs
// convention. Committed docs collection configs opt in explicitly; otherwise it
// follows the same code-repo detection used by init.
func DocsConventionActive(root string) bool {
	kind, err := DetectKind(root)
	if err == nil && kind == KindGitHubWiki {
		return false
	}
	folders, err := registeredDocFolders(root)
	if err == nil && len(folders) > 0 {
		return true
	}
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
	var hasDocsDir, hasNonDotDir bool
	var hasHome, hasWikiChrome, hasHyphenatedPage bool
	var mdCount, nonMdCount int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			switch name {
			case ".obsidian":
				hasObsidian = true
			case ".git":
				hasGit = true
			case "docs":
				hasDocsDir = true
				hasNonDotDir = true
			default:
				if !strings.HasPrefix(name, ".") {
					hasNonDotDir = true
				}
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
			lowerName := strings.ToLower(name)
			switch lowerName {
			case "home.md":
				hasHome = true
			case "_sidebar.md", "_footer.md":
				hasWikiChrome = true
			}
			if strings.Contains(strings.TrimSuffix(name, filepath.Ext(name)), "-") {
				hasHyphenatedPage = true
			}
		} else {
			nonMdCount++
		}
	}

	switch {
	case hasGitHubWikiSignal(dir):
		return KindGitHubWiki, nil
	case hasObsidian:
		return KindPlainVault, nil
	case hasSourceMarker:
		return KindCodeRepo, nil
	case isFlatGitHubWiki(hasDocsDir, hasNonDotDir, hasHome, hasWikiChrome, hasHyphenatedPage, mdCount, nonMdCount):
		return KindGitHubWiki, nil
	case mdCount > 0 && mdCount >= nonMdCount:
		return KindPlainVault, nil
	case hasGit:
		return KindCodeRepo, nil
	default:
		return KindPlainVault, nil
	}
}

func isFlatGitHubWiki(hasDocsDir, hasNonDotDir, hasHome, hasWikiChrome, hasHyphenatedPage bool, mdCount, nonMdCount int) bool {
	if hasDocsDir || hasNonDotDir || !hasHome || !hasWikiChrome || !hasHyphenatedPage {
		return false
	}
	return mdCount > 0 && mdCount >= nonMdCount
}

func hasGitHubWikiSignal(dir string) bool {
	if hasWikiSuffix(filepath.Base(dir)) {
		return true
	}
	cfg := gitConfigPath(dir)
	if cfg == "" {
		return false
	}
	f, err := os.Open(cfg)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "url") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && hasWikiSuffix(strings.TrimSpace(parts[1])) {
			return true
		}
	}
	return false
}

func gitConfigPath(dir string) string {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return filepath.Join(gitPath, "config")
	}
	raw, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(line, "gitdir:") {
		return ""
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}
	return filepath.Join(gitDir, "config")
}

func hasWikiSuffix(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimRight(value, "/")
	value = strings.TrimSuffix(value, ".git")
	return strings.HasSuffix(value, ".wiki")
}
