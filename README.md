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

Optional reranking: set `reranker_url` in `.stardust/config.toml` to a cross-encoder endpoint (e.g. `bge-reranker-v2-m3` served by `llama-server --reranking`, exposing `/v1/rerank`) to re-rank the top hybrid hits. Absent or unreachable, query serves raw hybrid results unchanged.

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
| `bundle <task> [--budget]` | assemble a task-scoped context bundle (PageRank-expanded, budgeted) |
| `remember <fact>` | store a fact in the vault (add-only, deduped into the nearest note) |
| `digest [--since] [--advance]` | summarize recent activity by area, with open commitments |
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

Routes: `GET /query`, `/note`, `/status`, `/graph`, `/bundle`, `/digest`, `/cron`, `/healthz`; `POST /index`, `/rebuild`, `/archive`, `/cron/run`. Full spec in [docs/openapi.yaml](docs/openapi.yaml).

Typed clients over the API live in [sdk/](sdk): a Go client (`sdk.New(url).Query(...)`) and a TypeScript client ([sdk/ts/stardust.ts](sdk/ts/stardust.ts), used by the Obsidian plugin).

## Claude Code (MCP)

`stardust serve --mcp` runs an MCP server over stdio exposing `query`, `get_note`, `status`, and `graph` tools, so agents can search your vault. It resolves the vault from the working directory or `STARDUST_VAULT`. A ready-made Claude Code plugin lives in [plugin/claude/](plugin/claude):

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

## Context bundles

`stardust bundle "<task>"` assembles the context an agent should boot with for a task: it seeds from hybrid recall, expands over the link graph with personalized PageRank (so notes *linked* to the matches come along, not just keyword/semantic matches), fuses with RRF, and packs the result to a token budget - the most relevant note's body plus summary lines for the rest, leaving headroom for just-in-time retrieval. Also at `GET /bundle?task=...` and as the MCP `bundle` tool.

## Write-back (agent memory)

Agents can co-author the vault. `stardust remember "<fact>"` embeds the fact, appends it to the most similar existing note (add-only) or creates a dated note under `memory/`, and re-derives the index so it is immediately searchable. Over MCP, the `remember` tool does the same, and the `memory` tool exposes the six edit verbs (`view`, `create`, `str_replace`, `insert`, `delete`, `rename`). All writes are confined to the vault root and serialized through a mutex, so concurrent agents cannot corrupt a file. Files stay the source of truth; the index follows.

## Temporal (digests + ambient agents)

Git is the change feed - no extra infrastructure. `stardust digest` summarizes what changed since the last cursor (or `--since`), grouped by area, and surfaces open commitments (TODO, "I'll do X") from the changed notes. `--advance` moves the cursor so the next digest is incremental. Wire it to a schedule with a cron job; see `docs/examples/cron-jobs/` for a `morning-digest` (a daily briefing) and a weekly `librarian` agent pass. Also at `GET /digest` and the MCP `digest` tool.

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

## Architecture

Everything sits on one core library (`internal/service`) over a vault. The CLI, the HTTP API, and the MCP server are thin frontends over it, so capability parity is structural - none can do anything the others cannot. The full design and the research behind it is in [SPEC.md](./SPEC.md).

The superpower layer (mounts, context bundles, write-back/memory, temporal digests), the surfaces (API, MCP + Claude Code plugin, Go + TS SDKs), and the Obsidian plugin are all built. Genuinely future work: a reranker served in-binary (pure-Go ONNX), LLM-based contradiction detection, and semantic mount routing at scale.
