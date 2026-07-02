# Stardust

[![CI](https://github.com/alxxpersonal/stardust/actions/workflows/ci.yml/badge.svg)](https://github.com/alxxpersonal/stardust/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Local-first, git-backed, markdown-truth context engine for AI agents. It indexes a markdown vault into a derived, rebuildable SQLite index (FTS5 keyword + local vector embeddings) and exposes hybrid search to humans (an interactive TUI) and agents (a scriptable CLI). Files stay the source of truth; the index is a disposable cache.

Full architecture and research notes: [SPEC.md](./SPEC.md). Release history: [CHANGELOG.md](./CHANGELOG.md).

## Install

```sh
brew install alxxpersonal/tap/stardust
# or with Go:
go install github.com/alxxpersonal/stardust/cmd/stardust@latest
# or from source, inside this repo:
make build && make install
```

Building from source needs Go 1.26+. Semantic (vector) search needs a local [Ollama](https://ollama.com) with an embedding model (`ollama pull bge-m3`). Without it, search degrades gracefully to FTS5 keyword-only.

Optional reranking: set `reranker_url` in `.stardust/config.toml` to a cross-encoder endpoint (e.g. `bge-reranker-v2-m3` served by `llama-server --reranking`, exposing `/v1/rerank`) to re-rank the top hybrid hits. Absent or unreachable, query serves raw hybrid results unchanged.

Wiki-to-code drift: set `source_root` in `.stardust/config.toml` when a GitHub wiki or docs vault documents a separate source repository. The value may be absolute or relative to the vault root. When a `governs:` path is not found in the vault, Stardust resolves it under `source_root` and counts source-repo commits after the wiki page was last touched.

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
| `new <name> [--template] [--check]` | scaffold a fresh vault: git init + `.stardust` + starter files + first commit |
| `new spec|plan|adr <title>` | scaffold convention docs with machine-readable YAML frontmatter |
| `init [--docs]` | scaffold `.stardust/`, manifest, INDEX.md, wire `core.hooksPath`; `--docs` also writes the specs, plans, adr, and research collections |
| `index [--since SHA] [--background]` | incremental reindex; content-hash skips unchanged, `--since` is the git-diff fast path |
| `query <text> [--limit N] [--output auto/md/json/plain]` | hybrid keyword + semantic search |
| `graph [--output ...]` | derive the link graph, report orphans and broken links |
| `check [--strict]` | validate vault integrity, docs conventions, `governs`, and agent `targets` |
| `bundle <task> [--budget]` | assemble a task-scoped context bundle (PageRank-expanded, budgeted) |
| `remember <fact>` | store a fact in the vault (add-only, deduped into the nearest note) |
| `digest [--since] [--advance]` | summarize recent activity by area, with open commitments |
| `registry [--output PATH]` | render the grouped, status-aware `docs/INDEX.md` from the docs collections |
| `registry governs <path>` | show specs, plans, ADRs, or research docs that govern a code path |
| `sync [--dry-run] [--check] [--repair]` | plan or apply skills and agents into Claude, Codex, and Gemini targets |
| `serve [--addr] [--mcp]` | run the HTTP/JSON API, or the MCP server over stdio with `--mcp` |
| `archive [--dest DIR]` | snapshot the vault's git history (timestamped bare mirror) |
| `cron list` / `cron run <job>` | list or run declarative cron jobs |
| `hooks install` / `hooks uninstall` | manage the auto-index commit hooks |
| `rebuild` | nuke and regenerate the entire cache |
| `version` | print the version |

Run with no arguments in a terminal to open the TUI. Pipe any command (non-TTY) for clean markdown or JSON, the agent surface. Add `--output json` for structured results.

## HTTP API

`stardust serve` runs a localhost HTTP/JSON API over the same core the CLI uses, so every capability is reachable programmatically (and by the Obsidian plugin). It has no auth and binds to `127.0.0.1` by default; keep it behind your own trust boundary.

```sh
stardust serve --addr 127.0.0.1:7777
curl 'http://127.0.0.1:7777/query?q=how+to+handle+errors&limit=5'
curl  http://127.0.0.1:7777/status
curl -X POST http://127.0.0.1:7777/index
```

Routes: `GET /query`, `/note`, `/status`, `/graph`, `/bundle`, `/digest`, `/cron`, `/mounts`, `/collections`, `/collection`, `/records`, `/record`, `/healthz`; `POST /index`, `/rebuild`, `/archive`, `/cron/run`, `/records`; `PATCH /record`. Full spec in [docs/openapi.yaml](docs/openapi.yaml).

Typed clients over the API live in [sdk/](sdk): a Go client (`sdk.New(url).Query(...)`) and a TypeScript client ([sdk/ts/stardust.ts](sdk/ts/stardust.ts), used by the Obsidian plugin).

## Claude Code (MCP)

`stardust serve --mcp` runs an MCP server over stdio exposing `query`, `get_note`, `status`, `graph`, and the collections tools (`list_collections`, `list_records`, `get_record`, `create_record`, `patch_record`), so agents can search and edit your vault. It resolves the vault from the working directory or `STARDUST_VAULT`. A ready-made Claude Code plugin lives in [plugin/claude/](plugin/claude):

```sh
claude plugin marketplace add ./plugin/claude
claude plugin install stardust@stardust-local
```

## Mounts (federate other sources)

A mount is any MCP server (a database, email, calendar, code host, ...) declared under `.stardust/mounts/<name>/config.toml`. `stardust query --mounts` fans the query out to every mount plus the local index and fuses the rankings with RRF, so one search spans your whole context, not just your notes. Stardust does not write connectors; it aggregates the MCP ecosystem's existing ones.

```toml
# .stardust/mounts/<name>/config.toml
command = "some-mcp-server"   # an executable stdio MCP server
args = ["serve"]
tool = "search"               # the downstream search tool (default "query")
[env]
API_KEY = "..."
```

A mount's search tool is called with `{ query, limit }`; results are read from a `hits` or `results` array (with `title` / `snippet` / `path` fields), or the raw text content. A failing mount is skipped, never failing the whole query. Also available over the API as `GET /query?mounts=true`.

## Collections (vault as a database)

A collection is a vault folder paired with a typed schema: folder = table, note = row, frontmatter = columns. Records are plain markdown notes already in the vault and the index, so a collection is just a structured view, not a second store. Declare one with a committed descriptor at `.stardust/collections/<name>/config.toml`:

```toml
# .stardust/collections/jobs/config.toml
path = "Jobs"                    # vault-relative folder the records live in
description = "job applications"

[[fields]]
name = "company"
type = "string"                  # string | number | bool | date | enum | tags | ref
required = true

[[fields]]
name = "status"
type = "enum"
enum = ["applied", "interview", "offer", "rejected"]

[[fields]]
name = "score"
type = "number"
```

Query records with frontmatter predicates (`field:op:value`, op one of `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `contains`, repeatable and combined with AND) and a sort field (a frontmatter key, or `path` / `updated_at`, prefixed with `-` for descending). Numeric values compare numerically:

```sh
curl -X POST http://127.0.0.1:7777/records \
  -H 'Content-Type: application/json' \
  -d '{"collection":"jobs","fields":{"company":"Acme","status":"applied","score":7},"body":"first lead"}'
curl 'http://127.0.0.1:7777/records?collection=jobs&where=status:eq:applied&sort=-score'
curl -X PATCH 'http://127.0.0.1:7777/record?path=Jobs/acme.md' \
  -H 'Content-Type: application/json' -d '{"fields":{"status":"interview"}}'
```

Create and patch validate fields against the schema (required fields, enum membership, basic types) and write the note via the path-confined memory store, then reindex - no git commit, matching the rest of the write-back layer. Reachable over the API (`/collections`, `/collection`, `/records`, `/record`), the MCP tools (`list_collections`, `list_records`, `get_record`, `create_record`, `patch_record`), and the SDK (`ListCollections`, `ListRecords`, `GetRecord`, `CreateRecord`, `PatchRecord`).

## Docs registry

`stardust registry` renders a grouped, status-aware table of contents for your docs from the docs collections, so the registry is owned by Stardust rather than hand-maintained. It queries the `specs`, `plans`, `adr`, and `research` collections through the same record query as everything else and writes one grouped markdown file. The command never errors on a missing or empty collection - it renders an empty section - and it is idempotent: running it twice produces no diff.

```sh
stardust init --docs       # scaffold the four docs collections under .stardust/collections/
stardust registry          # write the grouped docs/INDEX.md (override the path with --output)
stardust registry governs internal/service/check.go
stardust new spec "Agent Infra" --governs "internal/service/*.go"
stardust new plan "Agent Infra Rollout" --related docs/specs/2026-06-22-agent-infra.md
stardust new adr "Adopt Agent Sync"
```

Each group renders newest first by filename (ADRs sort by number ascending, in a numbered table). The output is regenerated, never hand-edited:

```markdown
# Docs Index

Generated by `stardust registry`. Do not edit by hand.

## specs

| Title | Status | Doc | Date |
|-------|--------|-----|------|
| Docs Convention and Versioning | Approved | docs/specs/2026-06-22-2238-docs-convention-and-versioning.md | 2026-06-22 |

## adr

| # | Title | Status | Doc |
|---|-------|--------|-----|
| 0001 | Adopt collections | Accepted | docs/adr/0001-adopt-collections.md |
```

The post-commit hook regenerates `docs/INDEX.md` on every commit (after the incremental index), so the registry cannot drift from the docs on disk. `stardust registry` also refreshes `.stardust/manifest.md` with active plans, stale implemented docs, and the core commands agents should see at boot.

## Agent sync

`stardust sync` discovers skills and agents from configured sources, routes them by frontmatter `targets`, and plans links or copies into Claude, Codex, and Gemini tool directories. Dry-run first, then use check mode as a drift gate:

```sh
stardust sync init --canonical a private skills repo --profile alxx --dry-run
stardust sync --dry-run --scope repo --tool claude
stardust sync --check --scope all
stardust sync --repair
stardust sync report
```

The sync config lives at `.stardust/sync.toml`. The alxx migration profile treats `a private skills repo/skills` and `a private skills repo/agents` as canonical, while `~/Code/Self/forge/skills`, `~/.agents/skills`, and loose `~/.claude` assets are import-only sources for the report.

```toml
default_targets = ["claude", "codex", "gemini"]

[[sources]]
name = "canonical-skills"
path = "a private skills repo/skills"
kind = "skill"
priority = 0

[[targets]]
tool = "claude"
scope = "repo"
skills_path = ".claude/skills"
agents_path = ".claude/agents"
mode = "symlink"
```

Per-asset routing is optional. Without `targets`, an item uses `default_targets`; with `targets`, it routes only to the named tools:

```yaml
---
name: skill-forge
description: Write or improve an LLM agent skill.
targets: [claude, codex]
---
```

Rules adapter sync ships. Author rules once in a canonical `.stardust/rules.md`, and `stardust sync` renders that body per tool through a format-aware adapter map, composing it into `CLAUDE.md`, `AGENTS.md`, and `GEMINI.md` as a sentinel-delimited managed block instead of a blind symlink. Compose owns only its block, so every line you write outside it survives each sync; a missing or stale block is reported by `stardust sync --check`, and a plain `stardust sync` self-heals compose drift without `--repair`.

## Context bundles

`stardust bundle "<task>"` assembles the context an agent should boot with for a task: it seeds from hybrid recall, expands over the link graph with personalized PageRank (so notes *linked* to the matches come along, not just keyword/semantic matches), fuses with RRF, and packs the result to a token budget - the most relevant note's body plus summary lines for the rest, leaving headroom for just-in-time retrieval. Also at `GET /bundle?task=...` and as the MCP `bundle` tool.

## Write-back (agent memory)

Agents can co-author the vault. `stardust remember "<fact>"` embeds the fact, appends it to the most similar existing note (add-only) or creates a dated note under `memory/`, and re-derives the index so it is immediately searchable. Over MCP, the `remember` tool does the same, and the `memory` tool exposes the six edit verbs (`view`, `create`, `str_replace`, `insert`, `delete`, `rename`). All writes are confined to the vault root and serialized through a mutex, so concurrent agents cannot corrupt a file. Files stay the source of truth; the index follows.

## Temporal (digests + ambient agents)

Git is the change feed - no extra infrastructure. `stardust digest` summarizes what changed since the last cursor (or `--since`), grouped by area, and surfaces open commitments (TODO, "I'll do X") from the changed notes. `--advance` moves the cursor so the next digest is incremental. Wire it to a schedule with a cron job; see `docs/examples/cron-jobs/` for a `morning-digest` (a daily briefing) and a weekly `librarian` agent pass. Also at `GET /digest` and the MCP `digest` tool.

## Vault health + scaffolding

`stardust check` validates vault integrity and conventions: broken wikilinks, malformed frontmatter, bad doc names, invalid doc statuses, broken `related` refs, `governs` patterns that match nothing, stale implemented specs, forbidden unicode dashes, and invalid agent `targets`. With `--strict` it exits non-zero on errors, so it can gate commits - `stardust hooks install --check strict` writes a pre-commit hook that blocks bad vault state (use `--check warn` to surface issues without blocking). Also at `GET /check` and the MCP `check` tool.

`stardust new <name>` bootstraps a fresh vault in one command: starter files (or `--template <dir>` to copy your own layout), `git init`, the `.stardust` scaffolding with hooks, and a first commit. Inside a vault, `stardust new spec|plan|adr <title>` creates convention docs in the configured docs collections and reindexes them immediately.

## Layout (`.stardust/` inside a vault)

```
config.toml        # committed: embed model, ollama url, ignore globs, source root
manifest.md        # committed: the always-pinned agent context (L0 keystone)
INDEX.md           # committed: generated table of contents
sync.toml          # committed: agent skill/source/target sync config
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

## Architecture

Everything sits on one core library (`internal/service`) over a vault. The CLI, the HTTP API, and the MCP server are thin frontends over it, so capability parity is structural - none can do anything the others cannot. The full design and the research behind it is in [SPEC.md](./SPEC.md).

The superpower layer (mounts, context bundles, write-back/memory, temporal digests), the surfaces (API, MCP + Claude Code plugin, Go + TS SDKs), and the Obsidian plugin are all built. Genuinely future work: a reranker served in-binary (pure-Go ONNX), LLM-based contradiction detection, and semantic mount routing at scale.
