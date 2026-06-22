package doclinks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGovernedPathMatch(t *testing.T) {
	root := t.TempDir()
	codePath := filepath.Join(root, "internal", "service", "check.go")
	if err := os.MkdirAll(filepath.Dir(codePath), 0o755); err != nil {
		t.Fatalf("create code dir: %v", err)
	}
	if err := os.WriteFile(codePath, []byte("package service\n"), 0o644); err != nil {
		t.Fatalf("write code: %v", err)
	}

	ok, matched, err := MatchGovernedPath(root, "internal/service/*.go", "internal/service/check.go")
	if err != nil {
		t.Fatalf("MatchGovernedPath() error = %v", err)
	}
	if !ok {
		t.Fatal("MatchGovernedPath() = false, want true")
	}
	if got, want := matched[0], "internal/service/check.go"; got != want {
		t.Fatalf("matched[0] = %q, want %q", got, want)
	}
}
