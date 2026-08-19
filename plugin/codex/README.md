# Stardust plugin for Codex CLI

Make Stardust the context spine of an OpenAI Codex CLI session: the Stardust MCP server, an auto-injected session-start policy plus live workspace state, and the docs authoring workflows as Agent Skills. This is the Codex-native sibling of the Claude Code plugin in [`../claude/`](../claude).

Verified end to end against `codex-cli 0.144.1`.

## What you get

- **MCP tools.** The `stardust` MCP server (`stardust serve --mcp`) exposes `bundle`, `query`, `remember`, `check`, `status`, `get_note`, `graph`, `digest`, `memory`, `mounts`, and the collection tools. Codex sees them prefixed as `stardust/<tool>`.
- **Session-start context.** On session start Codex runs the plugin's `SessionStart` hook, which injects the stardust-first routing policy and a live read-only `<workspace-state>` block (active plans, recent specs, `stardust check` counts, indexed HEAD). Requires a one-time hook trust step (see below).
- **Authoring skills.** Eight skills reproduce the Claude plugin's write workflows: `stardust-execute`, `stardust-spec`, `stardust-plan`, `stardust-doc`, `stardust-adr`, `stardust-audit`, `stardust-status`, `stardust-refresh`. They trigger by description or explicit invocation (Codex has no `$ARGUMENTS`-templated slash commands).

## Prerequisites

- The `stardust` binary on `PATH`. Install: `go install github.com/alxxpersonal/stardust/cmd/stardust@latest`.
- A Stardust workspace: a docs-convention repo (`stardust init --docs`) or a configured Obsidian vault.
- `codex-cli` recent enough to have `codex plugin` and the `hooks` feature (0.144.1 or newer verified).

## Install path A: the full plugin (recommended)

From the repo root (or any path to this bundle):

```sh
codex plugin marketplace add ./plugin/codex
codex plugin add stardust@stardust-codex-local
```

`codex plugin list` should then show `stardust@stardust-codex-local  installed, enabled`. Installing the plugin also registers the `stardust` MCP server (from `.mcp.json`); confirm with `codex mcp list`.

To install from GitHub instead of a local path:

```sh
codex plugin marketplace add alxxpersonal/stardust --sparse plugin/codex
codex plugin add stardust@stardust-codex-local
```

### Trust the session-start hook (one time)

New plugin command hooks are not trusted until you approve them, so the session-start context injection stays off until you do. In an interactive Codex session run `/hooks` and trust the Stardust `SessionStart` and `UserPromptSubmit` hooks. For non-interactive automation that already vets the source, `codex exec --dangerously-bypass-hook-trust` runs them without persisted trust. Until trusted, the MCP tools and skills still work; only the automatic context injection waits.

## Install path B: MCP only

If you want just the Stardust tools and not the hooks or skills:

```sh
codex mcp add stardust -- stardust serve --mcp
```

`codex mcp list` shows the `stardust` server. This is the smallest footprint; it drops the session-start injection and the authoring skills.

## Verify

```sh
codex plugin list | grep stardust                 # installed, enabled
codex mcp list | grep stardust                     # stardust  serve --mcp  enabled
codex exec -s read-only "Call the stardust status tool and report the index note count."
```

The last command exercises the MCP edge in a real session. In a hook-trusted session, the model also sees the injected `stardust-context-policy` block at the top of its context.

## Uninstall

```sh
codex plugin remove stardust@stardust-codex-local
codex plugin marketplace remove stardust-codex-local
codex mcp remove stardust        # only if you used install path B, or the entry lingers
```

## How it maps to the Claude plugin

| Claude plugin piece | Codex bundle piece | Notes |
|---|---|---|
| `.claude-plugin/plugin.json` | `.codex-plugin/plugin.json` | Native manifest with an `interface` block plus `mcpServers` and `skills` pointers |
| `.claude-plugin/marketplace.json` | `.claude-plugin/marketplace.json` | Codex reads the legacy marketplace descriptor; local single-plugin, source `./` |
| `.mcp.json` | `.mcp.json` | Byte-identical; `stardust serve --mcp` |
| `hooks/hooks.json` | `hooks/hooks.json` | Same shape; Codex normalizes `SessionStart` to `session_start` |
| `hooks/session-start.sh`, `prompt-submit.sh`, `scripts/resolve-root.sh` | copied verbatim | Codex exports `${CLAUDE_PLUGIN_ROOT}`, so the scripts run unchanged |
| `hooks/policy.txt` | `hooks/policy.txt` | Codex-neutralized: no `<user-instructions priority="override">` wrapper |
| `commands/*.md` (slash commands) | `skills/stardust-*/SKILL.md` | Codex removed custom prompts; workflows ship as Agent Skills |

## Limitations

- No `/stardust:spec`-style namespaced slash commands or positional `$ARGUMENTS`; skills take natural-language topics.
- The session-start injection lands as developer context, without the Claude "override" priority weight.
- Scripts are POSIX `sh`; Windows is not covered in v1.
- Maintenance and digest crons are not shipped as a Codex feature; run them at the OS level (cron or launchd) calling `stardust cron run <job>`.
- The three shared scripts are copied from `../claude`; keep them in sync when either bundle changes.
