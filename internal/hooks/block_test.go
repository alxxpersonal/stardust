package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const blockLines = "command -v stardust >/dev/null 2>&1 && stardust index >/dev/null 2>&1 || true\n" +
	"command -v stardust >/dev/null 2>&1 && stardust registry >/dev/null 2>&1 || true\n"

// countMarkers counts the start and end sentinel markers in body.
func countMarkers(body string) (starts, ends int) {
	return strings.Count(body, blockStart), strings.Count(body, blockEnd)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestInjectBlockCreatesFileWithShebang(t *testing.T) {
	path := filepath.Join(t.TempDir(), "post-commit")

	if err := injectBlock(path, blockLines); err != nil {
		t.Fatalf("injectBlock() error = %v", err)
	}

	body := readFile(t, path)
	if !strings.HasPrefix(body, "#!/bin/sh\n") {
		t.Fatalf("created file = %q, want a #!/bin/sh shebang prefix", body)
	}
	if starts, ends := countMarkers(body); starts != 1 || ends != 1 {
		t.Fatalf("markers = (%d start, %d end), want exactly one of each", starts, ends)
	}
	if !strings.Contains(body, "stardust index") {
		t.Fatalf("body = %q, want the injected lines", body)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0o755", info.Mode().Perm())
	}
}

func TestInjectBlockKeepsUserLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "post-commit")
	user := "#!/bin/sh\necho hello from the user\nnpx lint-staged\n"
	if err := os.WriteFile(path, []byte(user), 0o755); err != nil {
		t.Fatalf("write user hook: %v", err)
	}

	if err := injectBlock(path, blockLines); err != nil {
		t.Fatalf("injectBlock() error = %v", err)
	}

	body := readFile(t, path)
	if !strings.Contains(body, "echo hello from the user") {
		t.Fatalf("body = %q, want the user line preserved", body)
	}
	if !strings.Contains(body, "npx lint-staged") {
		t.Fatalf("body = %q, want the user line preserved", body)
	}
	if starts, ends := countMarkers(body); starts != 1 || ends != 1 {
		t.Fatalf("markers = (%d start, %d end), want exactly one of each", starts, ends)
	}
	// The user's shebang stays; injectBlock must not add a second one.
	if strings.Count(body, "#!/bin/sh") != 1 {
		t.Fatalf("body = %q, want exactly one shebang", body)
	}
}

func TestInjectBlockIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "post-commit")
	user := "#!/bin/sh\necho user line\n"
	if err := os.WriteFile(path, []byte(user), 0o755); err != nil {
		t.Fatalf("write user hook: %v", err)
	}

	if err := injectBlock(path, blockLines); err != nil {
		t.Fatalf("first injectBlock() error = %v", err)
	}
	first := readFile(t, path)

	if err := injectBlock(path, blockLines); err != nil {
		t.Fatalf("second injectBlock() error = %v", err)
	}
	second := readFile(t, path)

	if starts, ends := countMarkers(second); starts != 1 || ends != 1 {
		t.Fatalf("after two injects markers = (%d start, %d end), want exactly one of each", starts, ends)
	}
	if first != second {
		t.Fatalf("second inject changed the file:\nfirst:\n%q\nsecond:\n%q", first, second)
	}
	if !strings.Contains(second, "echo user line") {
		t.Fatalf("body = %q, want the user line preserved", second)
	}
}

func TestInjectBlockReplacesStaleBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "post-commit")

	if err := injectBlock(path, "echo old contents\n"); err != nil {
		t.Fatalf("first injectBlock() error = %v", err)
	}
	if err := injectBlock(path, "echo new contents\n"); err != nil {
		t.Fatalf("second injectBlock() error = %v", err)
	}

	body := readFile(t, path)
	if strings.Contains(body, "echo old contents") {
		t.Fatalf("body = %q, want the stale block replaced", body)
	}
	if !strings.Contains(body, "echo new contents") {
		t.Fatalf("body = %q, want the new block present", body)
	}
	if starts, ends := countMarkers(body); starts != 1 || ends != 1 {
		t.Fatalf("markers = (%d start, %d end), want exactly one of each", starts, ends)
	}
}

func TestStripBlockRemovesOnlyTheBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "post-commit")
	user := "#!/bin/sh\necho before\nnpx lint-staged\necho after\n"
	if err := os.WriteFile(path, []byte(user), 0o755); err != nil {
		t.Fatalf("write user hook: %v", err)
	}
	if err := injectBlock(path, blockLines); err != nil {
		t.Fatalf("injectBlock() error = %v", err)
	}

	if err := stripBlock(path); err != nil {
		t.Fatalf("stripBlock() error = %v", err)
	}

	body := readFile(t, path)
	if starts, ends := countMarkers(body); starts != 0 || ends != 0 {
		t.Fatalf("after strip markers = (%d start, %d end), want none", starts, ends)
	}
	if strings.Contains(body, "stardust index") {
		t.Fatalf("body = %q, want the stardust lines gone", body)
	}
	for _, want := range []string{"echo before", "npx lint-staged", "echo after"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body = %q, want the user line %q preserved", body, want)
		}
	}
	// Stripping must not leave a run of blank lines where the block was.
	if strings.Contains(body, "\n\n\n") {
		t.Fatalf("body = %q, want collapsed blank lines after strip", body)
	}
}

func TestStripBlockIsNoOpWithoutABlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "post-commit")
	user := "#!/bin/sh\necho only the user\n"
	if err := os.WriteFile(path, []byte(user), 0o755); err != nil {
		t.Fatalf("write user hook: %v", err)
	}

	if err := stripBlock(path); err != nil {
		t.Fatalf("stripBlock() error = %v", err)
	}

	if body := readFile(t, path); body != user {
		t.Fatalf("stripBlock() changed a file with no block:\ngot:  %q\nwant: %q", body, user)
	}
}

func TestStripBlockMissingFileIsNoError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist")
	if err := stripBlock(path); err != nil {
		t.Fatalf("stripBlock() on a missing file error = %v, want nil", err)
	}
}
