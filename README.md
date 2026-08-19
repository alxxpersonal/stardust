# Stardust

[![CI](https://github.com/alxxpersonal/stardust/actions/workflows/ci.yml/badge.svg)](https://github.com/alxxpersonal/stardust/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Local-first, git-backed, markdown-truth context engine for AI agents. It indexes a markdown vault into a derived, rebuildable SQLite index (FTS5 keyword + local vector embeddings) and exposes hybrid search to humans (an interactive TUI) and agents (a scriptable CLI, a JSON-RPC API, and an MCP server). Files stay the source of truth; the index is a disposable cache.

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

Optional reranking: a cross-encoder re-ranks the top hybrid hits when one is reachable, and it needs no endpoint config. Leave `reranker_url` empty in `.stardust/config.toml` and Stardust probes for a local runtime on the first query (an Ollama `/api/rerank` forward-compat seam, then a `llama-server --reranking` default at `http://localhost:8080/v1/rerank`), adopting the first that answers a canary. Set `reranker_url` to a specific endpoint to pin it, or to `none` to disable probing entirely. Absent or unreachable, query serves raw hybrid order unchanged and announces the source, so an inactive reranker is never mistaken for one that ran.

Wiki-to-code drift: point `source_root` in `.stardust/config.toml` at a separate source repository when a GitHub wiki or docs vault documents one (absolute, or relative to the vault root). When a `governs:` path is not found in the vault, Stardust resolves it under `source_root` and counts source-repo commits after the wiki page was last touched. For a `<name>.wiki` GitHub wiki checkout, `source_root` is optional: Stardust autodetects the sibling `../<name>` source repo when both git remotes canonicalize to the same GitHub repository, and refuses the bind on any doubt (a wrong bind would manufacture false drift). Wiki checkouts also index the non-markdown GitHub markup formats (AsciiDoc, reStructuredText, Textile, Org, Creole, MediaWiki, RDoc, Pod) as title-plus-plain-text, not just `.md`.

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
| `new spec\|plan\|adr <title>` | scaffold convention docs with machine-readable YAML frontmatter |
| `init [--docs] [--no-docs]` | scaffold `.stardust/`, manifest, INDEX.md, wire `core.hooksPath`; auto-detects a code repo and scaffolds the docs collections plus the `docs/agents/` home (override with `--docs` / `--no-docs`) |
| `status [--output auto/json]` | report vault initialization, detected kind, collections, and index health |
| `index [--since SHA] [--background]` | incremental reindex; content-hash skips unchanged, `--since` is the git-diff fast path |
| `query <text> [--limit N] [--output auto/md/json/plain]` | hybrid keyword + semantic search |
| `graph [--output ...]` | derive the link graph, report orphans and broken links |
| `check [--strict]` | validate vault integrity, docs conventions, `governs`, and agent `targets` |
| `indexes [--check]` | maintain opt-in per-directory `INDEX.md` files; `--check` reports stale ones without writing |
| `bundle <task> [--budget]` | assemble a task-scoped context bundle (PageRank-expanded, budgeted) |
| `remember <fact>` | store a fact in the vault (add-only, deduped into the nearest note) |
| `digest [--since] [--advance]` | summarize recent activity by area, with open commitments |
| `contradictions [--since] [--all] [--advance]` | surface cross-note contradiction candidates as review prompts (never verdicts) |
| `registry [--output PATH]` | render the grouped, status-aware `docs/INDEX.md` from the docs collections |
| `registry governs <path>` | show specs, plans, ADRs, or research docs that govern a code path |
| `sync [--dry-run] [--check] [--repair]` | materialize shared skills, subagents, and rules from `docs/agents/` into Claude, Codex, and Gemini; `--check` is a drift gate |
| `serve [--addr] [--mcp]` | run the JSON-RPC HTTP API (`POST /rpc`), or the MCP server over stdio with `--mcp` |
| `archive [--dest DIR]` | snapshot the vault's git history (timestamped bare mirror) |
| `cron list` / `cron run <job>` | list or run declarative cron jobs |
| `hooks install` / `hooks uninstall` | manage the commit hooks that reindex, regenerate the registry, and sync agent assets |
| `rebuild` | nuke and regenerate the entire cache |
| `version` | print the version |

Run with no arguments in a terminal to open the TUI: tabs for search, browse, graph, drift, and status, with a settings pane. Pipe any command (non-TTY) for clean markdown or JSON, the agent surface. Add `--output json` for structured results.

## JSON-RPC API

`stardust serve` runs a localhost HTTP server that exposes the whole core over one JSON-RPC 2.0 endpoint, `POST /rpc`. The same typed method registry backs the CLI, this HTTP bridge, and the MCP server, so the three surfaces are at structural parity by construction (ADR 0004, ADR 0006). It has no auth and binds to `127.0.0.1` by default; keep it behind your own trust boundary.

```sh
stardust serve --addr 127.0.0.1:7777
curl -s http://127.0.0.1:7777/rpc -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"query","params":{"query":"how to handle errors","limit":5}}'
curl -s http://127.0.0.1:7777/rpc -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"rpc.discover"}'   # the OpenRPC discovery document
curl -s http://127.0.0.1:7777/healthz                      # liveness probe (the only non-RPC route)
```

Methods: `status`, `query`, `bundle`, `graph`, `digest`, `check`, `note/get`, `index/run`, `index/rebuild`, `archive`, `cron/list`, `cron/run`, `mount/list`, `collection/list`, `collection/get`, `record/create`, `record/get`, `record/list`, `record/patch`, `record/delete`, `memory/remember`, `memory/edit`, and `rpc.discover`. The discovery document is generated from the live registry and committed at [docs/openrpc.json](docs/openrpc.json). The old REST paths were retired once the contract reached parity, so `GET /healthz` is the only route left beside `POST /rpc`.

Typed clients over the contract: a Go client in [rpc/](rpc) (`c := rpc.NewClient(url); c.Query(ctx, rpc.QueryParams{Query: "..."})`) covering every method, and a smaller TypeScript client in [sdk/ts/stardust.ts](sdk/ts/stardust.ts) used by the Obsidian plugin (a query, notes, and records subset over the same `POST /rpc`).

## Claude Code (MCP)

`stardust serve --mcp` runs an MCP server over stdio. Every tool call routes through the same JSON-RPC registry the CLI and HTTP API use (ADR 0003), so the MCP edge is a thin adapter, not a separate implementation. Tools exposed: `query`, `get_note`, `status`, `graph`, `bundle`, `mounts`, `check`, `digest`, `remember`, `memory`, and the collection tools (`list_collections`, `list_records`, `get_record`, `create_record`, `patch_record`). It resolves the vault from the working directory or `STARDUST_VAULT`. A ready-made Claude Code plugin lives in [plugin/claude/](plugin/claude):

```sh
claude plugin marketplace add ./plugin/claude
claude plugin install stardust@stardust-local
```

The plugin wires that MCP server, injects a stardust-first policy plus a live read-only workspace-state block at session start, arms maintenance and digest crons, and ships inline authoring commands: `/stardust:execute` (spec, plan, and build in one turn), `/stardust:spec`, `/stardust:plan`, `/stardust:doc`, `/stardust:adr`, `/stardust:audit` (a verified workspace audit written to `docs/research/`), plus `/stardust:setup`, `/stardust:status`, `/stardust:refresh`, and `/stardust:crons`.

## Codex CLI

A Codex-native plugin lives in [plugin/codex/](plugin/codex). It wires the same MCP server, injects the session-start policy and workspace state, and ships the authoring workflows as both slash commands (`/stardust:spec`, `/stardust:plan`, `/stardust:execute`, ...) and intent-triggered Agent Skills. Verified against `codex-cli 0.144.1`:

```sh
codex plugin marketplace add ./plugin/codex
codex plugin add stardust@stardust-codex-local
```

New plugin hooks need a one-time `/hooks` trust before the session-start injection fires. For just the tools, skip the plugin: `codex mcp add stardust -- stardust serve --mcp`. Full install, verification, and uninstall steps are in the plugin's own `plugin/codex/README.md`.

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

A mount's search tool is called with `{ query, limit }`; results are read from a `hits` or `results` array (with `title` / `snippet` / `path` fields), or the raw text content. A failing mount is skipped, never failing the whole query.

Query-aware routing keeps a large mesh fast: give a mount a `description` or `keywords` and Stardust fans a query out only to the mounts likely to hold the answer, judged semantically against the mount description (or lexically in FTS-only mode) before any subprocess launches. Routing is conservative by design (ADR 0042): a mount with no self-description is never pruned, an explicit `--mounts=a,b` scope always wins, and any ambiguity searches everything, so routing only saves work, it never silently drops a relevant source.

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

Query records with frontmatter predicates (`field:op:value`, op one of `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `contains`, repeatable and combined with AND) and a sort field (a frontmatter key, or `path` / `updated_at`, prefixed with `-` for descending). Numeric values compare numerically. Over JSON-RPC:

```sh
curl -s http://127.0.0.1:7777/rpc -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"record/create","params":{"collection":"jobs","fields":{"company":"Acme","status":"applied","score":7},"body":"first lead"}}'
curl -s http://127.0.0.1:7777/rpc -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"record/list","params":{"collection":"jobs","filter":[{"field":"status","op":"eq","value":"applied"}],"sort":"-score"}}'
```

Create and patch validate fields against the schema (required fields, enum membership, basic types) and write the note via the path-confined memory store, then reindex, no git commit, matching the rest of the write-back layer. Reachable through the JSON-RPC methods (`collection/list`, `collection/get`, `record/create`, `record/get`, `record/list`, `record/patch`, `record/delete`), the MCP tools (`list_collections`, `list_records`, `get_record`, `create_record`, `patch_record`), and the typed SDK clients.

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

## Agent assets (docs/agents)

Shared agent assets live in one place: `docs/agents/`. Drop a skill at `docs/agents/skills/<name>/SKILL.md`, a subagent at `docs/agents/subagents/<name>.md`, and shared rules at `docs/agents/rules.md`, and `stardust sync` materializes them into every configured AI tool directory (`.claude/`, `.codex/`, `.gemini/`) as symlinks or copies. `stardust init` scaffolds the area for a code repo.

The home is configurable: set `agents_dir` in `.stardust/config.toml` (empty or absent means `docs/agents`) to relocate the whole area, and every seam - the sync source defaults, the stray-doc and orphan exemptions, the registry Agents section, and the `init --docs` scaffold - follows the configured path, while `stardust check` rejects an absolute, repo-escaping, or collection-colliding value (ADR 0048).

**Placement is the sharing switch.** An asset under `docs/agents/` is shared to every configured tool. An asset dropped directly into a tool dir (`.claude/skills/` and friends) stays tool-local and is never touched by sync. The `targets:` frontmatter is an optional narrowing on top:

```yaml
---
name: skill-forge
description: Write or improve an LLM agent skill.
targets: [claude, codex]   # optional; omit to sync everywhere
---
```

**Stardust understands the area natively.** Because it lives under `docs/`, every asset is indexed, searchable, drift-tracked, and validated by the same checks as any note, and it is never flagged as a stray doc (ADR 0047). `stardust registry` lists the skills (name, description, targets) and the subagents in an Agents section, so one glance shows the whole cross-agent surface. Legacy repo-root `skills/`, `agents/`, and `.stardust/rules.md` remain real compat sources at lower precedence, so an existing layout keeps syncing until its files move into `docs/agents/` (a name defined in both places resolves to the `docs/agents/` copy).

**Sync runs itself.** The commit hooks re-run `stardust sync` on post-commit and post-merge (guarded, non-blocking, a no-op on a clean tree), and CI fails on drift with `stardust sync --check`. Hooks materialize symlinks on developer machines; CI verifies and cannot materialize, so the two roles never collide. Dry-run first, then treat check mode as the gate:

```sh
stardust sync --dry-run                # print the plan without writing
stardust sync                          # materialize into the tool dirs
stardust sync --check                  # exit non-zero on drift (the CI gate)
stardust sync --repair                 # repair drifted stardust-managed targets
```

Rules compose rather than clobber (ADR 0039). Author rules once in `docs/agents/rules.md`, and `stardust sync` renders that body per tool through a format-aware adapter, composing it into `CLAUDE.md`, `AGENTS.md`, and `GEMINI.md` as a sentinel-delimited managed block instead of a blind symlink. Compose owns only its block, so every line you write outside it survives each sync; a missing or stale block is reported by `stardust sync --check`, and a plain `stardust sync` self-heals compose drift without `--repair`. The sync config lives at `.stardust/sync.toml`; `stardust sync init` scaffolds it and `stardust sync report` lists adoptable assets from migration sources.

## Context bundles

`stardust bundle "<task>"` assembles the context an agent should boot with for a task: it seeds from hybrid recall, expands over the link graph with personalized PageRank (so notes *linked* to the matches come along, not just keyword/semantic matches), fuses with RRF, and packs the result to a token budget - the most relevant note's body plus summary lines for the rest, leaving headroom for just-in-time retrieval. Also over JSON-RPC (the `bundle` method) and as the MCP `bundle` tool.

## Write-back (agent memory)

Agents can co-author the vault. `stardust remember "<fact>"` embeds the fact, appends it to the most similar existing note (add-only) or creates a dated note under `memory/`, and re-derives the index so it is immediately searchable. Over MCP, the `remember` tool does the same, and the `memory` tool exposes the six edit verbs (`view`, `create`, `str_replace`, `insert`, `delete`, `rename`). All writes are confined to the vault root and serialized through a mutex, so concurrent agents cannot corrupt a file. Files stay the source of truth; the index follows.

## Temporal (digests, contradictions, ambient agents)

Git is the change feed, no extra infrastructure. `stardust digest` summarizes what changed since the last cursor (or `--since`), grouped by area, and surfaces open commitments (TODO, "I'll do X") from the changed notes. `--advance` moves the cursor so the next digest is incremental. Wire it to a schedule with a cron job; see `docs/examples/cron-jobs/` for a `morning-digest` (a daily briefing) and a weekly `librarian` agent pass. Also over JSON-RPC (the `digest` method) and the MCP `digest` tool.

`stardust contradictions` surfaces cross-note contradiction candidates: an assertion in one note ("we chose X") against an opposing same-subject line in another ("X is deprecated"), found deterministically from a polarity lexicon plus hybrid same-subject recall. It is LLM-free and runs entirely in the binary, and it never judges: every candidate is a review prompt ("a candidate, not a verdict, likely benign, confirm before acting"), so it never gates CI. Like digest it walks the change feed since a cursor (`--since` / `--all`, `--advance` to move it).

## Vault health + scaffolding

`stardust check` validates vault integrity and conventions: broken wikilinks, malformed frontmatter, bad doc names, invalid doc statuses, broken `related` refs, `governs` patterns that match nothing, stale implemented specs, forbidden unicode dashes, and invalid agent `targets`. With `--strict` it exits non-zero on errors, so it can gate commits: `stardust hooks install --check strict` writes a pre-commit hook that blocks bad vault state (use `--check warn` to surface issues without blocking). Also over JSON-RPC (the `check` method) and the MCP `check` tool.

`stardust status` prints a fast health probe without indexing: whether the vault is initialized, its detected kind (code repo, GitHub wiki, or plain vault), the source-root binding, collection record counts, and index freshness (vectors, reranker source, and commits behind HEAD).

`stardust indexes` maintains opt-in per-directory `INDEX.md` files declared under `[conventions.directory_indexes]`; `--check` reports missing or stale ones without writing, so it can gate alongside `check`.

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

The superpower layer (mounts with query-aware routing, context bundles, write-back/memory, temporal digests, contradiction candidates), the surfaces (the JSON-RPC API, the MCP server plus Claude Code plugin, the typed Go and TypeScript clients), and the Obsidian plugin are all built. Genuinely future work: a reranker served in-binary (pure-Go ONNX) so no local runtime is needed, and LLM-graded contradiction verdicts on top of the deterministic candidates.
