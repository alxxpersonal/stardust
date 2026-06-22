// Package doclinks matches docs convention governs patterns against repo paths.
package doclinks

import (
	"fmt"
	"path/filepath"
)

// MatchGovernedPath reports whether pattern governs path within root. The
// returned matches are repo-relative files matched by the pattern.
func MatchGovernedPath(root string, pattern string, path string) (bool, []string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return false, nil, fmt.Errorf("resolve root: %w", err)
	}
	cleanPath := cleanSlash(path)
	glob := filepath.Join(root, filepath.FromSlash(pattern))
	matches, err := filepath.Glob(glob)
	if err != nil {
		return false, nil, fmt.Errorf("glob governs pattern %q: %w", pattern, err)
	}
	var relMatches []string
	for _, match := range matches {
		rel, err := filepath.Rel(root, match)
		if err != nil {
			return false, nil, fmt.Errorf("rel governs match %s: %w", match, err)
		}
		rel = cleanSlash(rel)
		if rel == cleanPath {
			return true, []string{rel}, nil
		}
		relMatches = append(relMatches, rel)
	}
	return false, relMatches, nil
}

func cleanSlash(path string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
}
