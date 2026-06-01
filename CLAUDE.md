# Stardust

Local-first, git-backed, markdown-truth personal-context engine for AI agents. Full architecture: [SPEC.md](./SPEC.md).

## What it is

Indexes a markdown vault into a derived, rebuildable SQLite index (FTS5 + vectors) and exposes hybrid search to humans (charm TUI) and agents (CLI, later API/MCP). Obsidian stays the human editor; Stardust is the agent brain under the same files.

## Hard principles

- **Files as truth.** Markdown + frontmatter + git. Never put content in a bespoke store.
- **Derive, don't store.** The index, graph, and embeddings are disposable caches. `stardust rebuild` regenerates everything from markdown.
- **Dumb storage, genius wiring.** Boring proven engines (sqlite, FTS5, git, Ollama). Creativity goes into routing/fusion, never storage.
- **Match the tier to the scale.** One human, a few agents. sqlite, not Pinecone.

## Stack

- Go 1.26.1, module `github.com/alxxpersonal/stardust`. Layout: `cmd/stardust/main.go` (thin) + `internal/` only (no `pkg/`).
- cobra CLI, TOML config (`pelletier/go-toml/v2`).
- charm v2 TUI (`charm.land/...v2`): bubbletea, lipgloss, bubbles, huh, glamour.
- `modernc.org/sqlite` (pure Go, single static binary) + `pressly/goose` migrations. Vectors are brute-force cosine in Go (modernc cannot load the sqlite-vec C extension).
- Ollama for local embeddings (`bge-m3`), graceful fallback to FTS5-only when absent.

## Conventions

- See `.claude/rules/go.md`. Conventional commits, no co-author tags, no em/en dashes anywhere, `// --- Section ---` separators, doc comments on exports, `%w` error wrapping, never panic.
- Content hash (not mtime) is the index authority - cloud-synced storage rewrites mtimes.

## Deferred (see SPEC sections 8, 11, 12)

MCP server, HTTP/JSON API, full SDK, Obsidian plugin, and the superpower layer (mounts, context-bundles, write-back, temporal/ambient) are documented but not built in v1. The core library is structured so each is a thin later surface with full parity.
