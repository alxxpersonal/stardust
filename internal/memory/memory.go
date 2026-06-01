// Package memory implements the write-back primitive: the Anthropic six-verb
// memory tool (view/create/str_replace/insert/delete/rename) plus append, over
// vault markdown files. Every operation is path-confined to the vault root and
// serialized through a mutex so concurrent agents cannot read-modify-write the
// same file into corruption. The daemon (this Store) is the serialization point.
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store performs path-safe, serialized writes within a vault root.
type Store struct {
	root string
	mu   sync.Mutex
}

// New returns a memory store rooted at root.
func New(root string) *Store { return &Store{root: root} }

// safe resolves a vault-relative path and confines it to the root, rejecting any
// path that contains a parent-directory segment.
func (s *Store) safe(rel string) (string, error) {
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if seg == ".." {
			return "", fmt.Errorf("path escapes vault: %q", rel)
		}
	}
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+filepath.FromSlash(rel))), "/")
	if clean == "" || clean == "." {
		return "", fmt.Errorf("invalid path: %q", rel)
	}
	abs := filepath.Join(s.root, filepath.FromSlash(clean))
	rl, err := filepath.Rel(s.root, abs)
	if err != nil || rl == ".." || strings.HasPrefix(rl, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes vault: %q", rel)
	}
	return abs, nil
}

// View reads a file's contents.
func (s *Store) View(rel string) (string, error) {
	abs, err := s.safe(rel)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("view %s: %w", rel, err)
	}
	return string(b), nil
}

// Create writes a new file, failing if it already exists (add-only friendly).
func (s *Store) Create(rel, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := s.safe(rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err == nil {
		return fmt.Errorf("create %s: already exists", rel)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return fmt.Errorf("create %s: %w", rel, err)
	}
	return nil
}

// Append adds text to the end of a file, creating it if absent.
func (s *Store) Append(rel, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := s.safe(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("append %s: %w", rel, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(text); err != nil {
		return fmt.Errorf("append %s: %w", rel, err)
	}
	return nil
}

// StrReplace replaces a unique occurrence of oldStr with newStr.
func (s *Store) StrReplace(rel, oldStr, newStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := s.safe(rel)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("str_replace %s: %w", rel, err)
	}
	content := string(b)
	switch strings.Count(content, oldStr) {
	case 0:
		return fmt.Errorf("str_replace %s: text not found", rel)
	case 1:
		return os.WriteFile(abs, []byte(strings.Replace(content, oldStr, newStr, 1)), 0o644)
	default:
		return fmt.Errorf("str_replace %s: text is not unique", rel)
	}
}

// Insert inserts text as a new line before the given 0-based line index.
func (s *Store) Insert(rel string, line int, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := s.safe(rel)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("insert %s: %w", rel, err)
	}
	lines := strings.Split(string(b), "\n")
	if line < 0 || line > len(lines) {
		return fmt.Errorf("insert %s: line %d out of range", rel, line)
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:line]...)
	out = append(out, text)
	out = append(out, lines[line:]...)
	return os.WriteFile(abs, []byte(strings.Join(out, "\n")), 0o644)
}

// Delete removes a file.
func (s *Store) Delete(rel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := s.safe(rel)
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil {
		return fmt.Errorf("delete %s: %w", rel, err)
	}
	return nil
}

// Rename moves a file from oldRel to newRel, both confined to the vault.
func (s *Store) Rename(oldRel, newRel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	oldAbs, err := s.safe(oldRel)
	if err != nil {
		return err
	}
	newAbs, err := s.safe(newRel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newAbs), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return fmt.Errorf("rename %s: %w", oldRel, err)
	}
	return nil
}
