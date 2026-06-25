package hooks

import (
	"fmt"
	"os"
	"strings"
)

// Sentinel markers that delimit stardust's managed block inside a hook file. They
// match ADR 0008 exactly: install replaces what is between them, uninstall strips
// from blockStart through blockEnd, and lines outside them are never touched.
const (
	blockStart = "# >>> stardust >>> (managed block, do not edit)"
	blockEnd   = "# <<< stardust <<<"
)

// shebang is written when injectBlock has to create the hook file from scratch.
const shebang = "#!/bin/sh\n"

// injectBlock adds stardust's managed block (lines, wrapped in the sentinel
// markers) to the hook file at path with a single read-modify-write. If the file
// is absent it is created with a #!/bin/sh shebang. If the block is already
// present it is replaced in place, so repeated calls are idempotent and never
// duplicate the block. Lines outside the markers are preserved untouched. The
// file is left mode 0o755 so git treats it as an executable hook.
func injectBlock(path, lines string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read hook %s: %w", path, err)
		}
		existing = []byte(shebang)
	}

	body := stripExistingBlock(string(existing))
	block := blockStart + "\n" + ensureTrailingNewline(lines) + blockEnd + "\n"
	next := ensureTrailingNewline(body) + block

	if err := os.WriteFile(path, []byte(next), 0o755); err != nil {
		return fmt.Errorf("write hook %s: %w", path, err)
	}
	return nil
}

// stripBlock removes stardust's managed block from the hook file at path,
// collapsing the blank lines left where the block was. A file without the block
// and a missing file are both left untouched and reported as success.
func stripBlock(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read hook %s: %w", path, err)
	}

	body := string(existing)
	stripped := stripExistingBlock(body)
	if stripped == body {
		return nil
	}

	if err := os.WriteFile(path, []byte(stripped), 0o755); err != nil {
		return fmt.Errorf("write hook %s: %w", path, err)
	}
	return nil
}

// stripExistingBlock returns body with stardust's managed block removed. It drops
// every line from the blockStart marker through the blockEnd marker (inclusive)
// and collapses the resulting run of blank lines into a single one so the file
// does not accumulate gaps across install/uninstall cycles. Body with no block is
// returned unchanged.
func stripExistingBlock(body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == blockStart:
			inBlock = true
		case inBlock && trimmed == blockEnd:
			inBlock = false
		case inBlock:
			// drop lines inside the block
		default:
			out = append(out, line)
		}
	}
	return collapseBlankRuns(strings.Join(out, "\n"))
}

// collapseBlankRuns squeezes any run of two or more blank lines into a single
// blank line, leaving the rest of the text intact.
func collapseBlankRuns(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// ensureTrailingNewline returns s with exactly one trailing newline, so blocks
// append on their own line. An empty string stays empty.
func ensureTrailingNewline(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimRight(s, "\n") + "\n"
}
