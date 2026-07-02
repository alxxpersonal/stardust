package convention

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/alxxpersonal/stardust/internal/config"
)

// --- Source-root resolution ---

// Source-binding origins reported by ResolveSourceRoot.
const (
	// SourceOriginConfigured marks a source root taken verbatim from config.
	SourceOriginConfigured = "configured"
	// SourceOriginDetected marks a source root autodetected from a sibling checkout.
	SourceOriginDetected = "detected"
)

// ResolveSourceRoot resolves the source-repo root that cross-repo wiki-to-code
// drift binds against, returning the absolute path and its origin. It is the one
// seam every source-root consumer routes through so check, drift, and governs
// bind identically.
//
// Precedence is configured > detected > none. An explicit source_root always
// wins: a non-empty value is returned via config.ResolveSourceRoot with origin
// SourceOriginConfigured and no probing, even when it is wrong or missing on
// disk. When source_root is unset, a <name>.wiki GitHub wiki workspace probes
// exactly the sibling ../<name>; a confirmed same-repo checkout binds with
// origin SourceOriginDetected. Any doubt returns "", "", nil, byte-identical to
// today's empty-source-root behavior. A wrong bind manufactures false drift on
// every page, so detection is conservative to the point of refusing whenever it
// cannot positively confirm the target.
func ResolveSourceRoot(cfg config.Config, root string) (string, string, error) {
	if strings.TrimSpace(cfg.SourceRoot) != "" {
		path, err := cfg.ResolveSourceRoot(root)
		if err != nil {
			return "", "", err
		}
		return path, SourceOriginConfigured, nil
	}
	if sibling := detectSiblingSourceRoot(root); sibling != "" {
		return sibling, SourceOriginDetected, nil
	}
	return "", "", nil
}

// detectSiblingSourceRoot returns the absolute sibling source checkout for a
// <name>.wiki GitHub wiki workspace, or "" when any condition fails. Every one of
// the six conditions is required: the basename carries the .wiki suffix, the
// directory detects as a GitHub wiki, the stripped name is non-empty, the sibling
// ../<name> exists and is a directory, the sibling is a git checkout, and the two
// git remotes canonicalize to the same GitHub repository.
func detectSiblingSourceRoot(root string) string {
	base := filepath.Base(root)
	if !hasWikiSuffix(base) {
		return ""
	}
	if kind, err := DetectKind(root); err != nil || kind != KindGitHubWiki {
		return ""
	}
	name := stripWikiSuffix(base)
	if name == "" {
		return ""
	}
	sibling := filepath.Join(filepath.Dir(root), name)
	info, err := os.Stat(sibling)
	if err != nil || !info.IsDir() {
		return ""
	}
	if gitConfigPath(sibling) == "" {
		return ""
	}
	if !sameRepoIdentity(remoteURL(root), remoteURL(sibling)) {
		return ""
	}
	return filepath.Clean(sibling)
}

// stripWikiSuffix reduces a <name>.wiki directory basename to <name>, mirroring
// hasWikiSuffix's trimming of a trailing slash and .git while preserving the
// original case of the remaining name. It returns "" when the value does not
// carry the .wiki suffix or strips to empty.
func stripWikiSuffix(base string) string {
	trimmed := strings.TrimRight(base, "/")
	if len(trimmed) >= len(".git") && strings.EqualFold(trimmed[len(trimmed)-len(".git"):], ".git") {
		trimmed = trimmed[:len(trimmed)-len(".git")]
	}
	if len(trimmed) < len(".wiki") || !strings.EqualFold(trimmed[len(trimmed)-len(".wiki"):], ".wiki") {
		return ""
	}
	return trimmed[:len(trimmed)-len(".wiki")]
}

// remoteURL returns the first git-remote url configured for dir, or "" when dir
// has no git config or no url line. It mirrors the config scan hasGitHubWikiSignal
// already performs.
func remoteURL(dir string) string {
	cfg := gitConfigPath(dir)
	if cfg == "" {
		return ""
	}
	f, err := os.Open(cfg)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "url") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "url" {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// sameRepoIdentity reports whether two git remotes name the same GitHub
// repository. It requires both canonical identities to be non-empty and equal, so
// a missing remote on either side is treated as no match.
func sameRepoIdentity(wikiURL, srcURL string) bool {
	wiki := canonicalRepoIdentity(wikiURL)
	src := canonicalRepoIdentity(srcURL)
	return wiki != "" && src != "" && wiki == src
}

// canonicalRepoIdentity reduces a git remote url to a scheme-and-user-independent
// host/owner/repo identity: it lowercases, trims a trailing slash, drops a leading
// scheme, drops a user@ host prefix, rewrites the scp-form host:owner/repo colon
// to a slash, then strips a trailing .git and a trailing .wiki. It returns "" for
// an empty or unusable url. Reducing both sides this way makes a wiki remote
// (.../owner/repo.wiki.git) and its source remote (.../owner/repo.git) compare
// equal regardless of transport.
func canonicalRepoIdentity(url string) string {
	s := strings.ToLower(strings.TrimSpace(url))
	if s == "" {
		return ""
	}
	s = strings.TrimRight(s, "/")
	for _, scheme := range []string{"https://", "http://", "ssh://", "git://"} {
		if strings.HasPrefix(s, scheme) {
			s = s[len(scheme):]
			break
		}
	}
	if at := strings.IndexByte(s, '@'); at >= 0 {
		if slash := strings.IndexByte(s, '/'); slash < 0 || at < slash {
			s = s[at+1:]
		}
	}
	if colon := strings.IndexByte(s, ':'); colon >= 0 {
		s = s[:colon] + "/" + s[colon+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, ".wiki")
	return strings.Trim(s, "/")
}
