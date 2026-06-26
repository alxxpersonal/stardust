# Stardust - Claude Code plugin

Makes stardust the context spine of a Claude Code session. It wires the stardust MCP server,
injects a stardust-first policy plus live workspace state at session start, and arms native
crons for index, registry, sync, and digests. One definition serves both an Obsidian vault
and a docs-convention code repo.

## What it does

- Wires the stardust MCP server (`query`, `bundle`, `remember`, `get_note`, `graph`, `check`,
  and the collection tools) so the model can route context in-process.
- Injects, at SessionStart, a static stardust-first policy followed by a small, read-only
  `<workspace-state>` block: active plans, recent specs, verification counts, and the next
  step. The whole emission targets under 5 KB and never reindexes at boot.
- Steers the model to `stardust bundle` and `stardust query` for plans and decisions, and
  reserves grep, the editor, and the language server for source code.
- Arms two native crons on demand: maintenance (index, registry, sync) and a daily digest.
- Provides authoring commands that resolve the workspace and author docs directly in the
  current turn.
- Degrades to a silent no-op, or a single one-time pointer, when stardust is absent or no
  workspace resolves. It never errors loudly and never nags.

## Prerequisites

The `stardust` binary must be on your `PATH`:

```sh
go install github.com/alxxpersonal/stardust/cmd/stardust@latest
# then, in your repo or vault:
stardust init --docs && stardust index
```

`jq` is used by the hook scripts and is present on macOS and most Linux installs.

## Install (local development)

```sh
claude plugin marketplace add ./plugin/claude
claude plugin install stardust@stardust-local
```

The MCP server and the SessionStart hook wire automatically. Commands are namespaced under
the plugin name `stardust`, so they are invoked as `/stardust:setup`, `/stardust:crons`,
`/stardust:refresh`, `/stardust:status`, `/stardust:spec`, `/stardust:plan`,
`/stardust:doc`, and `/stardust:adr`.

## Modes

The hook scripts resolve one workspace per session via `scripts/resolve-root.sh`:

1. Repo mode: if `${CLAUDE_PROJECT_DIR}/.stardust` exists, the project root is used. Zero
   config.
2. Vault mode: otherwise, if `config.json` names a `vaultPath` that exists on disk (with
   `mode` set to `auto` or `vault`), that vault is used.
3. Otherwise: no workspace, and the plugin stays quiet.

## Configure

Repo mode needs no configuration. Vault mode is one command:

```
/stardust:setup
```

It records the vault path (from `STARDUST_VAULT` or a prompt), runs `stardust init --docs` if
needed, runs the first index, and writes `config.json` under the plugin data directory.

`${CLAUDE_PLUGIN_DATA}/config.json` shape:

```json
{
  "mode": "auto",
  "vaultPath": "<absolute path to your vault>",
  "digestHourLocal": 8,
  "maintenanceCron": "0 */2 * * *",
  "midConversationReminders": false
}
```

- `mode`: `auto` resolves an initialized repo first, then the vault; `vault` or `repo` force
  one.
- `vaultPath`: absolute path to the Obsidian vault.
- `digestHourLocal`: local hour, 0 to 23, for the daily digest cron.
- `maintenanceCron`: five-field, local-time schedule for the maintenance cron.
- `midConversationReminders`: when true, the prompt-submit hook may emit one debounced
  retrieval reminder per window. Off by default.

## Commands

### Operations

- `/stardust:setup` `[vault|repo]` - configure vault mode or confirm repo mode, then run the
  first index.
- `/stardust:crons` - arm the maintenance and digest crons as native scheduled tasks, in
  local time. Crons are created only when you run this command, never at install.
- `/stardust:refresh` - re-index the resolved workspace and regenerate `docs/INDEX.md`.
- `/stardust:status` - show the resolved mode, root, and index health.

### Authoring

These run the write side of the docs loop, the counterpart to the read-side state injected at
session start. Each resolves the workspace, surfaces relevant existing docs, writes the
convention-correct file set, and regenerates `docs/INDEX.md` (`allowed-tools: Bash, Read,
Write`).

- `/stardust:spec` `[what to spec]` - start a technical spec, ADRs, and implementation plan
  using the inline spec workflow.
- `/stardust:plan` `[topic to plan, or empty to list]` - list active plans from `docs/plans`,
  or write a new spec and plan using the inline spec workflow.
- `/stardust:doc` `[adr|research|runbook] [topic]` - add one ADR, research note, or runbook to
  `docs/` using the inline single-doc workflow.
- `/stardust:adr` `[decision to document]` - record an architectural decision as an ADR via
  the inline ADR workflow.

The authoring commands embed the forge workflows directly so a command such as
`/stardust:plan plan this feature` writes the durable spec, ADRs, plan, and registry update in
one turn. They keep `docs/plans/` canonical, never write `docs/superpowers/`, and do not use
the unrelated Microsoft `.docx` tooling.

## Crons and the 7-day expiry

Native recurring crons auto-expire after 7 days. The maintenance cron self-heals by calling
`CronList` and re-creating any plugin cron that is missing or near expiry, so the schedule
survives as long as a session runs at least weekly. If no session runs for over 7 days,
re-run `/stardust:crons`. Native crons cap at 50 per session.

## Graceful degradation

| Condition | Behavior |
|---|---|
| stardust not on PATH | SessionStart prints nothing, exits 0. One-time install pointer at most. |
| Resolved root has no `.stardust/` | One-time pointer to `/stardust:setup`, then silence. |
| Ollama down | stardust falls back to keyword-only search. Nothing breaks. |
| `vaultPath` missing on disk (not synced yet) | Resolves to none; a quiet no-op. |
| MCP server fails to start | The CLI affordance still works; the policy references CLI verbs too. |
| Crons at the 50 cap | `/stardust:crons` reports it and exits; the commit hook keeps repo mode fresh. |

## Files

```
.claude-plugin/plugin.json   manifest
.mcp.json                    wires the stardust MCP server
hooks/hooks.json             SessionStart + UserPromptSubmit registration
hooks/session-start.sh       resolves mode/root, emits policy + live state
hooks/prompt-submit.sh       optional, debounced retrieval reminder
hooks/policy.txt             the static stardust-first policy (cacheable)
commands/setup.md            configure dual-mode, init + first index
commands/crons.md            arm the maintenance and digest crons
commands/refresh.md          manual index + registry
commands/status.md           show resolved mode, root, index health
commands/spec.md             write a spec, ADRs, and plan inline
commands/plan.md             list active plans or write a spec and plan inline
commands/doc.md              write one ADR, research note, or runbook inline
commands/adr.md              write one ADR inline
scripts/resolve-root.sh      shared mode/root resolution
```

Logging from the MCP server goes to stderr; stdout is reserved for the JSON-RPC protocol.
