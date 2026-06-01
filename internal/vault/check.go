package vault

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Problem is a single-file vault-integrity issue.
type Problem struct {
	Kind   string // bad-frontmatter | missing-title
	Detail string
}

// CheckFile validates one markdown file: that any frontmatter block is valid
// YAML, and that the note has an explicit title (a frontmatter title or an H1).
func CheckFile(root, rel string) ([]Problem, error) {
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	body := string(raw)

	var problems []Problem
	hasFrontmatterTitle := false
	if m := frontmatterRe.FindStringSubmatch(body); m != nil {
		fm := map[string]any{}
		if err := yaml.Unmarshal([]byte(m[1]), &fm); err != nil {
			problems = append(problems, Problem{Kind: "bad-frontmatter", Detail: "frontmatter is not valid YAML"})
		} else if fmString(fm, "title") != "" {
			hasFrontmatterTitle = true
		}
		body = body[len(m[0]):]
	}
	if !hasFrontmatterTitle && !h1Re.MatchString(body) {
		problems = append(problems, Problem{Kind: "missing-title", Detail: "no frontmatter title and no H1 heading"})
	}
	return problems, nil
}
