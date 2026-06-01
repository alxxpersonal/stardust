# Stardust - Vision & Architecture

Status: draft / blueprint. Date: 2026-06-01.
A v2 architecture, learning from an earlier overengineered graph-DB attempt.
Named after the Terraria Stardust pillar - the summoner class: you command a swarm of minions (agents).

---

## 1. Thesis

**Dumb storage, genius wiring, derive everything else.**

Stardust is a local-first personal context system for AI agents. The source of truth stays plain markdown + git. Every index, graph, and embedding is a *derived, disposable artifact* rebuilt from the files. The intelligence lives in the agentic layer - retrieval, curation, and (the superpower layer) federating every data source you own into one context an agent can reach. It is the agent brain under the same files Obsidian already edits, not a replacement for Obsidian.

It is explicitly NOT a database you sync. A prior attempt - a stateful custom graph DB exposed over MCP - failed by being overengineered. The lesson: never put the content in a bespoke store; keep files as truth and treat everything else as a cache.

## 2. Governing principles

1. **Files as truth.** Markdown + frontmatter + git. Greppable, diffable, portable, human- and agent-readable, survives any tool dying. Git history is a free non-lossy temporal log (you get mem0's add-only + Graphiti's invalidate-not-delete for free).
2. **Derive, don't store.** Graph, index, embeddings = rebuildable artifacts, never a stateful DB to keep in sync. `stardust rebuild` regenerates the entire derived layer from markdown in one command. Structurally impossible to drift.
3. **Dumb parts, genius wiring.** Use boring proven engines (sqlite-vec, FTS5, ripgrep, git, Ollama, existing MCP connectors). The creativity goes into routing/fusion/agent-workflows, never into rebuilding storage.
4. **Match the tier to the scale.** One human + a handful of agents. sqlite-vec, not Pinecone+Neo4j. Don't cosplay a FAANG data platform for personal notes.
5. **Agents must not be able to NOT know it exists.** Discoverability is a context-engineering problem, not a retrieval problem (see L0). Solve that first.

## 3. Architecture (layers)

### L0 - Discoverability (BUILD FIRST, before any RAG)
The core pain ("spawn an agent and it doesn't even know the vault exists") is tool-affordance, not retrieval quality. Three stacked mechanisms:
- **Always-pinned manifest** (Letta/MemGPT "core memory"): a ~30-line high-signal block injected into every agent + subagent boot - what the vault is, where the index lives, the ONE search tool + when to use it, the most load-bearing conventions. A pinned block is structurally un-ignorable. Fixes ~80% of the felt problem alone. Keep it <50 lines (bloated context degrades reliability).
- **The v1 affordance is the CLI, NOT MCP.** An agent reaches Stardust by running `stardust query "..."` through its existing Bash tool (clean markdown out, via dual-mode), plus a generated `INDEX.md` it greps. The manifest just says "to search your context, run `stardust query`." This is MCP-FREE, works today with ANY shell-having agent (Claude Code, codex, cursor - not just MCP clients), needs no SDK/server/`.mcp.json`, and the research rates the index-first + grep pattern as reliable-or-better than a fancy MCP tool.
- **MCP is DEFERRED to the mounts/superpower layer**, packaged as a Claude Code plugin (`stardust serve --mcp`). MCP's real payoff is the connector ecosystem (the mounts), which is a later feature - so MCP arrives WITH mounts, not in v1. When built: ONE MCP tool, sharp WHEN/WHEN-NOT description, git-committed `.mcp.json` (NOT resources - they don't auto-load).
- **Hooks + subagent templates** reinforce the CLI path too: `SessionStart` guarantees awareness, and every spawned subagent uses a template that RE-declares the manifest + the `stardust query` instruction (subagents inherit neither skills nor the parent prompt - this is the actual bug). Reference for the eventual MCP plugin: `engraph` (github.com/devwhodevs/engraph).

### L1 - Source of truth
Markdown + frontmatter + git. Untouched. Obsidian remains the human editor on the same files.

### L2 - Derived index (retrieval engine)
One SQLite file = **sqlite-vec (vectors) + FTS5 (BM25) + a catalog table**. Deletable, rebuildable. Retrieval recipe in ROI order:
1. **Hybrid search** (vector + BM25 fused with RRF, k=60). Biggest win: +26-31% over pure vector (BM25 nails wikilink names / codenames / dates that embeddings blur). Build first.
2. **Local cross-encoder rerank** (bge-reranker-v2-m3 or Qwen3-Reranker-0.6B) over top ~50-100 -> 5-10. Second-biggest win, fully local.
3. **Markdown-header chunking + parent-document** (embed sections, return the whole note). Skip semantic chunking (NAACL 2025: fixed chunks match it for far less cost). Skip HyDE (hallucinates on your own facts).
4. **Local embeddings via Ollama** - bge-m3 or nomic-embed-text (8192-ctx). DO NOT use mxbai-embed-large (512-token cap silently truncates long notes).

Reference impls to steal: **qmd** (github.com/tobi/qmd - Shopify CEO's local markdown search; sqlite-vec + FTS5 + local rerank + MCP + incremental; basically Stardust already), memweave, echovault, Continue's indexer.

### L3 - Graph (derived, NOT a DB)
Do NOT build Microsoft GraphRAG (100-1000x cost, no cheap incremental, ICLR 2026 found it underperforms vanilla RAG on local questions - the same overengineering trap). Instead **derive the graph for free from `[[wikilinks]]` + frontmatter relations** (regex + YAML, <2s, $0, yields MORE edges than LLM extraction: 10.4 vs 7.3 per file). Use for 1-2 hop neighbor expansion at retrieval time + orphan/broken-link detection. Steal Graphiti's semantics (validity intervals -> a `superseded_by` frontmatter field), not its machinery.

### L4 - Pipeline (keep it fresh, cheap)
Git-diff is the spine: `git diff --name-only <last_indexed_sha>..HEAD` -> for each changed `.md`, chunk + content-hash + upsert only changed chunks (delete-then-insert per file, prune deletes). Content-hash catalog means re-runs are no-ops and renames re-embed nothing.
- **CRITICAL gotcha:** cloud-synced storage (iCloud, Dropbox, etc.) rewrites mtimes -> key off CONTENT HASH, not mtime, or sync churn triggers garbage re-embeds.
- Triggers: git post-commit hook (instant/local) + a CI action on push (Forgejo / GitHub Actions / GitLab CI rebuilds index + INDEX.md, commits back). Same `stardust index` code path, multiple entrypoints.

### L5 - Self-maintaining (makes it feel like memory)
A scheduled/idle **librarian agent** pass: dedup near-identical notes, link orphans, summarize clusters into evergreen notes, flag stale via `last_verified`, auto-route the void. Passive index -> active memory.

## 4. The `.stardust/` convention

**Split the TOOL (global) from per-vault STATE.** Do not clone the tool into each vault (that's cloning git's source into every `.git/`).

- **The tool** = its own repo (any git remote), built into ONE binary installed globally (`go install` -> `$GOBIN/stardust`). Shared by every vault.
- **`.stardust/` in a vault** = per-vault config + cache. Split by committed (travels with clones) vs gitignored (rebuildable cache):

```
stardust/                         # the CLI repo - remote, global binary, NOT in the vault

<vault>/.stardust/
  config.yaml          # embed model, paths, ignore globs            [committed]
  manifest.md          # the always-pinned agent context (L0 keystone) [committed]
  mcp.json             # points every agent at `stardust serve`       [committed]
  INDEX.md             # generated table-of-contents                  [committed]
  hooks/               # versioned git hooks (core.hooksPath target)  [committed]
  mounts/<name>/config.yaml      # external source connectors (superpower L) [committed]
  cron-jobs/agents/
    vault-gardener/
      prompt.md        # what the agent does                         [committed]
      config.yaml      # trigger (schedule | on:commit), model, perms, scope [committed]
    void-triager/ ...
  cache/               # <---- gitignored, rebuildable
    db.sqlite          # sqlite-vec + FTS5 index + embeddings
    graph.json         # derived link-graph
    catalog            # content-hash ledger
```

Rule: **committed = convention/source (config, manifest, agent prompts, mounts). gitignored = derived cache (sqlite, graph, catalog).** The cache rebuilds from markdown; never sync a binary blob through cloud storage or git.

## 5. CLI surface

- `stardust init` - scaffold `.stardust/` in any vault, wire `core.hooksPath`
- `stardust index [--since <sha>] [--background]` - incremental reindex (git-diff + content-hash)
- `stardust query "..."` - hybrid + rerank retrieval (the backend for the MCP tool)
- `stardust serve` - the MCP server agents connect to (discoverability)
- `stardust graph` - build the derived link-graph
- `stardust cron list | run <job>` - read `cron-jobs/agents/*/config.yaml`, fire `codex exec --sandbox read-only` with that job's `prompt.md`
- `stardust rebuild` - nuke + regenerate the whole cache
- `stardust archive [--dest <folder>]` - one of the core functions: copies/snapshots the full `.git` (history included) and optionally the working tree into a desired destination folder (NAS, archive dir, external drive), timestamped. The passive git-backup primitive - a `git clone --mirror` / tarball of `.git` you can run on a schedule, a hook, or by hand. Pairs with the restic/NAS layer in Section 10 (this is the local-first leg of the backup story, agent-runnable).

The daemon/launchd timer just loops `stardust cron run` per the schedules. Even cron orchestration is "read the files, fire codex" - declarative, no hidden state.

**Core + three surfaces (full parity, well-documented).** Everything lives in ONE `stardust` core library (go package) that does the actual work. Three THIN frontends sit over it with full capability parity *by construction* - none can do anything the others can't, because they all call the same core:
- **CLI** (`stardust ...`) - humans, scripts, cron.
- **HTTP/JSON API** (`stardust serve --api`) - programmatic; the Obsidian plugin, other tools, remote/NAS access. Documented with an OpenAPI spec.
- **MCP server** (`stardust serve --mcp`) - agents.

One core = one source of truth. The CLI is core + flag-parsing, the API is core + HTTP handlers, the MCP is core + tool schemas - so there is nothing to keep in sync and every capability is reachable from every surface. All three are documented: CLI `--help`/man pages, OpenAPI for the API, MCP tool descriptions for agents. "Anything you can do anywhere, you can do everywhere."

**v1 scope (built 2026-06-01): the CLI surface only.** The HTTP/JSON API, the MCP server, and a **full SDK** (a typed client over the HTTP API, for the Obsidian plugin + external tools) are DEFERRED. The core library is already factored so each is a thin later addition with full parity - the API is core + handlers, the SDK is a generated/hand-written client over that API, the MCP is core + tool schemas. Build the API first, then the SDK on top of it, then the MCP plugin (which arrives with mounts).

## 6. Cron-job convention (the unified scheduler)

`.stardust/cron-jobs/` is the single declarative scheduler for everything periodic - it runs ALL stardust commands AND external commands AND agents. Each job = a folder + `config.yaml` (+ `prompt.md` for agent jobs). A job is `{ trigger, run }`:

- **trigger**: `schedule: "<cron>"` (periodic) OR `on: commit` + `when: { paths: [...] }` (event-driven).
- **run** is one of three kinds:
  - `agent` - fire `codex exec --sandbox <mode>` with this folder's `prompt.md` (gardener, void-triager, briefer...)
  - `command` - run a **stardust subcommand** (`index`, `archive --dest /nas`, `rebuild`). So maintenance + backup ARE cron jobs.
  - `exec` - run an arbitrary **external command** (`restic backup`, a sync script). So the scheduler covers non-stardust ops too.

```yaml
# vault-gardener/config.yaml      (agent, periodic)
trigger: { schedule: "0 3 * * *" }
run: { agent: { prompt: prompt.md, model: codex, sandbox: read-only } }

# nightly-archive/config.yaml     (stardust command)
trigger: { schedule: "0 4 * * *" }
run: { command: "archive --dest /nas/stardust-archives" }

# nightly-backup/config.yaml      (external)
trigger: { schedule: "30 4 * * *" }
run: { exec: "restic backup /vault" }

# void-triager/config.yaml        (agent, event-driven)
trigger: { on: commit, when: { paths: ["inbox/**"] } }
run: { agent: { prompt: prompt.md, model: codex, sandbox: read-only } }
```

The daemon reads every `config.yaml`, schedules the `schedule:` jobs, and the post-commit hook fires the `on: commit` jobs with matching path filters. Each run logs to `.stardust/cron-jobs/<name>/runs/<ts>.log` for observability. **Keep agent jobs OFF the per-commit path by default** - commit hook = fast mechanical indexing; agents are periodic/event-opt-in (or you melt the sub + slow every save). A mutating `command`/`exec` (anything prod-touching) declares its own safety and never auto-fires unattended without an explicit opt-in.

## 7. Commit-hook wiring

- **Version the hooks** via `git config core.hooksPath .stardust/hooks` (committed, travel to clones; `.git/hooks/` is untracked). Chain politely with existing husky/.githooks rather than clobbering.
- **Async, non-blocking, never fail the commit:** `stardust index --since HEAD~1 --background || true`. The index is a cache; a stale cache is fine, a blocked commit is not. If the NAS daemon is up, the hook just enqueues.
- **One indexer, three triggers:** post-commit (local/instant), post-merge/post-rewrite (re-index after pull), Forgejo Action on push (reproducible full-rebuild + commit-back). Identical code path.

## 8. The superpower layer (what makes it crazy, not just good)

The base (L0-L5) makes the vault searchable - table stakes. The superpower is that **your notes stop being the boundary.** (Implementation research in progress - see Section 12.)

1. **Mounts / context mesh (the killer).** Stardust is a router over EVERY source you own, each mounted as an MCP connector: a Postgres database, email, calendar, chat, code repos, finances, the web. One agent query fans out -> vault notes + DB rows + recent emails + recent commits + open invoices -> fused + reranked -> one unified context. The vault becomes a personal context OS over your whole digital life. Each mount is just a connector Stardust aggregates - thin, declarative (`.stardust/mounts/<name>/config.yaml`).
2. **Context bundles - agents boot loaded, not blank.** When you spawn an agent for a task, Stardust pre-assembles the exact context bundle (relevant notes + DB rows + recent activity + manifest) so the agent wakes up already holding what it needs. The actual "superpower in terms of context."
3. **Write-back - living agent memory.** Agents co-author: learn something -> write it to the RIGHT note, link it, index updates. Shared across agents + sessions (one team brain). Stops being "a DB I query," becomes "memory."
4. **Temporal + proactive/ambient.** Git history + mount activity gives agents a sense of time ("what changed this week," "this contradicts that," "you said you'd do X 3 weeks ago"). Flip pull -> push: morning briefings, pre-meeting context, contradiction flags.
5. **Provenance + freshness on every chunk** - source, written-when, verified-when, confidence. Agents reason far better knowing "stale note never re-checked" vs "live from the DB."

**Discipline guardrail:** the crazy lives in ROUTING, FUSION, AGENT WORKFLOWS - never in rebuilding storage. Mounts are connectors you orchestrate; bundles are queries you assemble; memory is markdown you write. Dumb parts, genius wiring. This is the line that keeps Stardust from becoming an overengineered bespoke DB.

**Positioning:** not an Obsidian replacement. Obsidian stays the human editor + graph view; Stardust is the agent brain under the same markdown; an optional thin Obsidian plugin surfaces Stardust search + gardener suggestions inline. Human layer + agent layer on one substrate.

## 9. Go / Rust

- **Go** owns the control plane: daemon, file-watch, git ops, the MCP server, CLI, scheduling. Single static binary, goroutines for parallel embedding, lives on the always-on NAS. Calls sqlite-vec (C) + Ollama (embeddings/rerank).
- **Rust** only if profiling ever finds a real bottleneck (custom vector index / inference loop) - probably never, because qdrant (rust) + sqlite-vec already exist.
- (Research note: Python is the "boring-correct default" for the pipeline specifically via LlamaIndex's free incremental upsert; go is the right call if you value the single-binary-on-NAS deploy.)

### Implementation stack (2026-06-01)

Recommended stack:
- **Go 1.26.1**, module `github.com/alxxpersonal/stardust`. Layout: `cmd/stardust/main.go` (thin) + `internal/` only (NO `pkg/`). `main` calls `cli.Execute()`, errors to stderr + `os.Exit(1)`.
- **CLI: cobra** + pflag. **Config: TOML** via `pelletier/go-toml/v2` (so consider TOML for the per-job configs too, for consistency - the `config.yaml` examples above would become `config.toml`).
- **TUI: the charm v2 stack** (note the `charm.land/...v2` import paths, not `charmbracelet/`): `bubbletea/v2` (Elm model/update/view), `lipgloss/v2`, `bubbles/v2` (table/textinput/viewport/spinner), `huh/v2` (forms), `glamour/v2` (markdown -> styled terminal), `harmonica` (animation). A proven production TUI stack.
- **MCP: `github.com/modelcontextprotocol/go-sdk v1.5.0`** - the official MCP Go SDK, so `stardust serve --mcp` is native.
- **Storage: pure-Go `modernc.org/sqlite`** + `pressly/goose` migrations. **GOTCHA:** modernc is pure-Go (no CGO = single static binary, trivial cross-compile) but it CANNOT load the sqlite-vec C extension. So either (a) keep pure-Go modernc + FTS5 (BM25) and do vectors **brute-force in Go** - at personal scale (hundreds-to-low-thousands of notes x ~768 dims) a flat cosine scan is instant and preserves the single-binary win, **recommended**; or (b) switch to CGO `mattn/go-sqlite3` to load sqlite-vec only if the corpus outgrows brute force. Default to (a).
- **Logging `log/slog`** (status -> stderr, structured -> stdout). **testify** + `-race -count=1`. **golangci-lint** + gofmt. **git-cliff** (cliff.toml). Conventional commits, no co-author tags, no em/en dashes, `// --- Section ---` separators, doc comments on exports.

### Dual-mode output (human TUI vs agent markdown)
The CLI serves humans AND agents from the same commands:
- **TTY -> interactive charm TUI** with glamour-rendered markdown via a custom `ansi.StyleConfig` + a per-width render cache.
- **non-TTY (piped/agent) -> clean markdown or JSON to stdout.** Detect via `isatty.IsTerminal(os.Stdout.Fd())` and pass `tea.WithoutRenderer()` when non-interactive (charm's `tui-daemon-combo` pattern), or the implicit "no args = TUI, subcommand = structured output" rule. Add `--output md|json|plain` to force it.

This is exactly the "markdown for agent work, interactive for human" dual surface.

## 10. Hosting & backup

- **Vault**: local-first on your workstation, git-backed. Code stays on a git host; the vault never leaves your control in plaintext.
- **NAS**: double duty - private git remote (`git push nas`) + restic target for the full file-level backup (incl `.obsidian`, assets, `.git`). Can run Forgejo (Docker) = a self-hosted git host + the CI that runs the reindex on push. The go daemon lives here (always-on, no laptop dependency).
- **One encrypted offsite leg**: NAS replicates its restic backup (encrypted) to Backblaze B2 / a second location. Survives total site loss, still 100% yours because it's ciphertext. NAS-only is governed-but-fragile; encrypted-offsite = governed AND disaster-proof.
- **3-2-1 satisfied**: local machine + NAS + 2nd git remote + restic->B2.
- Only back up the SOURCE (files + git + assets). Skip the derived index/graph/embeddings - rebuildable for free.

## 11. Build order

1. **L0 affordance (MCP-FREE)** - pinned manifest + `stardust query` via Bash + generated INDEX.md + subagent template. Ship nothing else first; it's the whole felt problem. No MCP yet.
2. **Hybrid sqlite index + git-diff incremental + commit hook.**
3. **Local rerank.**
4. **Derived link-graph** (neighbor expansion).
5. **Librarian agent** (cron).
6. **Superpower layer**: mounts first (biggest leap, and where MCP ENTERS - as a Claude Code plugin), then context bundles, then write-back, then temporal/proactive. Then the Obsidian plugin.

Each layer is independently useful and rebuildable.

**Built in v1 (2026-06-01):** L0 manifest + INDEX.md, the hybrid sqlite index (FTS5 + brute-force vectors + RRF, git-diff/content-hash incremental), `init` / `index` / `query` / `graph` / `archive` / `rebuild` / `cron` / `hooks`, the derived link-graph, the declarative cron scheduler (agent | command | exec), commit-hook wiring, and the full charm-v2 multi-tab TUI (search / status / graph) with dual-mode output.

**Built post-v1:** optional cross-encoder rerank (`internal/rerank`, `reranker_url`); the `internal/service` core seam + the HTTP/JSON API (`stardust serve`, `internal/api`, `docs/openapi.yaml`) - CLI and API now share one implementation, full parity by construction.

**Deferred (documented, not built):** a **full SDK**; the MCP server (arrives with mounts as a Claude Code plugin); the Obsidian plugin; and the entire superpower layer (mounts, context bundles, write-back, temporal/ambient - Sections 8 and 12). Order when resumed: MCP -> mounts -> bundles -> write-back -> temporal -> SDK -> plugin.

## 12. Superpower layer - implementation (researched 2026-06-01)

The base (L0-L5) makes the vault searchable. The superpower layer (Section 8) is the holy-shit tier - and the research found thin, proven implementations for all of it. Nothing to rebuild.

### 12.1 Mounts / context mesh
- **Thin MCP aggregator, NOT an enterprise gateway.** Mount N MCP servers (postgres, gmail, calendar, discord, github, fs) behind ONE endpoint via the Servers -> Namespaces -> Endpoints model. Stardust wants PROXY + ROUTER behavior (protocol mediation + dispatch); explicitly REFUSE the GATEWAY tier (OAuth/RBAC/audit = enterprise tax, zero payoff for one trusted human). Reference: MarimerLLC/mcp-aggregator (thin, lazy-load, JSON registry, no DB), tbxark/mcp-proxy. You do NOT write connectors - the MCP ecosystem already covers every source.
- **Don't preload tools - discover them.** Dumping all mount tool-defs in context is the killer (standard MCP eats up to 72% of context; tool-selection accuracy collapses 43% -> 14% as the menu grows). Expose ONE search/discovery tool over the mounts (Stardust's manifest + MCP tool IS this pattern, validated). Anthropic Tool Search (GA) auto-triggers in Claude Code past 10% context. Reuse the sqlite-vec index for tool descriptions too.
- **Fuse cross-source with RRF.** A postgres row, a gmail thread, a markdown note, a discord msg merge into one ranking via Reciprocal Rank Fusion (k=60) - rank-based, so ZERO calibration across wildly different sources. The single most reusable thin primitive; already how hybrid search fuses, generalizes to N mounts for free. At this scale, fan-out-to-all-mounts + RRF beats a query router.
- **Strategic call:** winning pattern = markdown+git truth + ONE derived sqlite index + federate live sources as MCP mounts the agent fans out to + RRF fusion. NOT Glean/Onyx "ingest everything" (fights derive-don't-store). Reference proof: blakecrosley's Obsidian MCP build.

### 12.2 Context bundles (the agent-superpower)
A thin Go **context assembler** between a task spec and an agent boot, fusing the two rankers Stardust already owns:
1. **Seed** from the task (cheap, no LLM): the cron-agent's `prompt.md` + `config` (declared tags, `[[wikilinks]]`, topic, folder scope) ARE the query.
2. **Rank with both substrates + fuse**: (a) **personalized PageRank over the derived link-graph** seeded with the task nodes (the Aider repomap move, ported to Go - the single highest-leverage steal); (b) **hybrid sqlite-vec + FTS5 recall -> cross-encoder rerank**. Fuse with RRF. Apply a frontmatter **metadata pre-filter** (status/recency/folder) BEFORE rerank to keep the pool tight.
3. **Pack to a fill-% budget**, not absolute tokens: manifest + task-goal pinned at HEAD, top-K wikilink summaries (lightweight identifiers, not full bodies), the single most-relevant note's body at the TAIL, 30-50% headroom for the agent's just-in-time pulls. Hybrid preload + JIT (Anthropic): preload a curated bundle, leave the MCP search tool open for the rest.
- **Gotcha - context rot:** a bigger bundle = WORSE outcomes past a threshold (Chroma 18-model study: lost-in-the-middle + distractor interference). The assembler MINIMIZES tokens at fixed task-success; "did every item earn its tokens" is the acceptance test.

### 12.3 Write-back / shared memory
- **Verb contract:** expose the Anthropic memory-tool six verbs (view/create/str_replace/insert/delete/rename) as Stardust MCP tools; the go daemon executes every op, enforces path safety, and re-derives the index + link-graph after each write. (Already Stardust's model - just formalize the contract.)
- **Routing + dedup-BEFORE-write** (the "land the fact in the right note" layer): never blind-append. Embed the candidate (Ollama) -> sqlite-vec nearest notes/sections -> LLM picks append-to-existing vs create-new vs skip. Resolve the target by exact > fuzzy > semantic match (Graphiti ladder).
- **Conflict policy: ADD-ONLY + invalidate-not-delete.** mem0 retreated from UPDATE/DELETE (error-prone, lossy) to ADD-only; Graphiti/Zep mark superseded not deleted. Prefer appending a new fact + stamping the old `superseded_by` (frontmatter `valid_from`/`valid_to`) over risky in-place rewrites; let retrieval+rerank surface the current version at READ time. Git already gives the ingestion-time axis for free - only model valid-time in frontmatter.
- **Multi-agent = blackboard.** The vault IS the blackboard; cron-agents are the knowledge sources; the pinned manifest + MCP discovery is the awareness layer. Per-region isolation: shared task/cron state = lock-and-claim, per-agent findings = append-only (no lock), private scratch = no lock. The **daemon is the serialization point** for shared-region writes.
- **Gotchas:** last-write-wins is a trap (NTP/clock skew makes the wrong write "win" - serialize through the daemon, never resolve by mtime). Silent corruption reads as hallucination (two agents read-modify-write the same note, one clobbers, the corrupt note poisons every downstream reader).

### 12.4 Temporal / proactive / ambient
- **Git IS the event stream** - the highest-leverage realization. No Kafka/CDC: `git log --since=<last-run> --name-only` + the daemon holding a "last processed commit SHA" cursor IS the change-feed, with ground-truth what-changed-and-when for free. The derived index already knows recency.
- **Ambient agents** (LangChain): agents listen to the change-feed and act unprompted, surfacing via three trust modes - **notify** (flag, don't act), **question** (ask when info missing), **review** (draft, wait for approval). Your codex-exec folder-agents BECOME ambient the moment a change-feed triggers them instead of you.
- **Three high-value behaviors:** (1) morning digest grouped by project via the link-graph (Anthropic `digest` skill blueprint: group by project not source, lead with action items, graceful-degrade on missing sources); (2) commitment surfacing (grep changed notes for TODO/"I'll do X", resurface stale ones); (3) contradiction detection across notes.
- **Trigger choice:** cron-diff-git for scheduled agents (digest, weekly); fswatch/FSEvents only if you want instant live-while-editing. Ship cron-diff first.

### 12.5 Obsidian plugin
Surface the local Stardust backend inside Obsidian (keep it the human editor): a thin plugin calls Stardust's local HTTP API / CLI, renders a search-results + chat panel and gardener suggestions inline. Pattern proven by Smart Connections / Khoj / Copilot-for-Obsidian (all integrate a local service). Do not fork Obsidian.

### Superpower references
MarimerLLC/mcp-aggregator, tbxark/mcp-proxy, MetaMCP, Anthropic Tool Search + effective-context-engineering + memory-tool + digest-skill, Aider repomap (PageRank), Chroma context-rot study, mem0, Graphiti/Zep, LangChain ambient agents, blakecrosley Obsidian MCP.

## 13. Key references & gotchas (from research)

References: qmd (github.com/tobi/qmd), engraph (github.com/devwhodevs/engraph), memweave, echovault, Continue indexer, Khoj, LightRAG (github.com/HKUDS/LightRAG), Letta/MemGPT, Anthropic memory tool + effective-context-engineering, mem0, Zep/Graphiti.

Gotchas: mxbai-embed-large 512-token cap silently truncates; HyDE hallucinates on personal facts (use multi-query expansion instead); semantic chunking is oversold (NAACL 2025); cloud sync rewrites mtimes -> use content hash; deletes/renames need explicit pruning; resources are a trap (use MCP tools); CLAUDE.md is ignorable on its own (pin the manifest); subagents inherit neither skills nor parent prompt (re-declare in the template); MS GraphRAG has no cheap incremental update (don't build it).
