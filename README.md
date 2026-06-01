# Stardust

Local-first, git-backed, markdown-truth context engine for AI agents. It indexes a markdown vault into a derived, rebuildable SQLite index (FTS5 keyword + local vector embeddings) and exposes hybrid search to humans (an interactive TUI) and agents (a scriptable CLI). Files stay the source of truth; the index is a disposable cache.

Full architecture and research notes: [SPEC.md](./SPEC.md).

## Install

```sh
go install github.com/alxxpersonal/stardust/cmd/stardust@latest
# or, inside this repo:
make build && make install   # builds ./build/stardust and go-installs it
```

Requires Go 1.26+. Semantic (vector) search needs a local [Ollama](https://ollama.com) with an embedding model (`ollama pull bge-m3`). Without it, search degrades gracefully to FTS5 keyword-only.

## Quickstart

```sh
cd /path/to/your/vault
stardust init       # scaffold .stardust/, write the manifest, wire commit hooks
stardust index      # build the search index from markdown
stardust query "how do I handle errors in go"
stardust            # no args in a terminal: launch the interactive TUI
```

## Commands

| command | what it does |
|---|---|
| `init` | scaffold `.stardust/`, manifest, INDEX.md, wire `core.hooksPath` |
| `index [--since SHA] [--background]` | incremental reindex; content-hash skips unchanged, `--since` is the git-diff fast path |
| `query <text> [--limit N] [--output auto/md/json/plain]` | hybrid keyword + semantic search |
| `graph [--output ...]` | derive the link graph, report orphans and broken links |
| `archive [--dest DIR]` | snapshot the vault's git history (timestamped bare mirror) |
| `cron list` / `cron run <job>` | list or run declarative cron jobs |
| `hooks install` / `hooks uninstall` | manage the auto-index commit hooks |
| `rebuild` | nuke and regenerate the entire cache |
| `version` | print the version |

Run with no arguments in a terminal to open the TUI. Pipe any command (non-TTY) for clean markdown or JSON, the agent surface. Add `--output json` for structured results.

## Layout (`.stardust/` inside a vault)

```
config.toml        # committed: embed model, ollama url, ignore globs
manifest.md        # committed: the always-pinned agent context (L0 keystone)
INDEX.md           # committed: generated table of contents
hooks/             # committed: versioned git hooks
cron-jobs/<name>/  # committed: config.toml (+ prompt.md for agent jobs)
cache/             # gitignored, rebuildable: db.sqlite, graph.json
```

Committed = convention and source. Gitignored = derived cache, rebuilt from markdown by `stardust index` / `stardust rebuild`.

## Cron jobs

Each job is a folder with a `config.toml`:

```toml
[trigger]
schedule = "0 4 * * *"        # or: on = "commit", paths = ["inbox/**"]

[run]
kind = "command"              # command | exec | agent
command = "archive --dest /nas/stardust-archives"
```

`command` runs a stardust subcommand, `exec` runs an external shell command, `agent` runs `codex exec` with the folder's `prompt.md`. A launchd or cron timer drives schedules by calling `stardust cron run <job>`; each run is logged under `cron-jobs/<name>/runs/`.

## Stack

Go 1.26, cobra CLI, TOML config, the charm v2 TUI stack (bubbletea, lipgloss, bubbles, glamour), `modernc.org/sqlite` (pure Go, single static binary) with goose migrations, and Ollama for local embeddings. Vectors are brute-force cosine in Go because pure-Go sqlite cannot load the sqlite-vec C extension; at personal scale a flat scan is instant and keeps the single static binary.

## Deferred

The HTTP/JSON API, a full client SDK, the MCP server (a Claude Code plugin), the Obsidian plugin, and the superpower layer (mounts, context bundles, write-back, temporal agents) are designed in [SPEC.md](./SPEC.md) but not built in v1. The core library is factored so each is a thin later surface with full parity.
