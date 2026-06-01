// Package index is the derived retrieval engine: a single rebuildable SQLite
// file holding the chunk catalog, an FTS5 (BM25) table, and per-chunk vectors.
// It is a disposable cache over the markdown source of truth.
package index

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps the sqlite index. Every method takes a context and uses
// parameterised queries.
type Store struct {
	db *sql.DB
}

// Open opens (creating parent dirs as needed) the sqlite index at path, applies
// pragmas, runs pending migrations, and returns a ready Store. Callers must Close.
func Open(ctx context.Context, path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create index dir %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := applyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := runMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close sqlite: %w", err)
	}
	return nil
}

// --- Meta ---

// GetMeta returns the value for key, or an empty string if the key is absent.
func (s *Store) GetMeta(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get meta %s: %w", key, err)
	}
	return v, nil
}

// SetMeta upserts a meta key/value pair.
func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	if err != nil {
		return fmt.Errorf("set meta %s: %w", key, err)
	}
	return nil
}

// --- Helpers ---

func applyPragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, stmt := range pragmas {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply %s: %w", stmt, err)
		}
	}
	return nil
}

func runMigrations(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("run migrations up: %w", err)
	}
	return nil
}
