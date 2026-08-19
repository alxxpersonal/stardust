---
title: Codex reuses the Claude hook scripts and neutralizes only policy.txt
status: Accepted
date: 2026-07-13
---

# 0051 - Codex reuses the Claude hook scripts and neutralizes only policy.txt

## Context

The Claude plugin injects session context with three POSIX scripts: `hooks/session-start.sh` (emits `policy.txt` plus a live `<workspace-state>` block), `hooks/prompt-submit.sh` (a debounced retrieval reminder), and `scripts/resolve-root.sh` (workspace resolution). Verified against `codex-cli 0.144.1`: plugin `hooks/hooks.json` loads in Codex (the `[hooks.state]` table tracks `session_start` and `user_prompt_submit` hooks from installed plugins), Codex exports `${CLAUDE_PLUGIN_ROOT}` and `${CLAUDE_PLUGIN_DATA}` to hook processes, and event names normalize from Claude PascalCase to Codex snake_case. The scripts are env- and filesystem-driven, with no Claude-API calls. Only `policy.txt` carries a Claude-specific framing: it opens with `<user-instructions priority="override">`, and Codex injects hook stdout as plain developer context, so that "override" weight does not carry.

## Decision

Copy `session-start.sh`, `prompt-submit.sh`, and `resolve-root.sh` into `plugin/codex/` byte-for-byte. Ship a Codex-neutralized `policy.txt` that drops the `<user-instructions priority="override">` wrapper and keeps the tool-neutral routing guidance (which already steers to "the bundle and query tools" without hardcoding any `mcp__stardust__` tool name). Register both hooks in `plugin/codex/hooks/hooks.json` with the same Claude-shape config.

## Consequences

- Zero rewrite of the hook logic; the deterministic steer to `stardust bundle`/`query` and the live workspace-state injection port directly.
- The "override" priority is lost on Codex; accepted, since the routing content still lands as context.
- Three scripts are duplicated across `plugin/claude/` and `plugin/codex/` (drift risk); mitigated by copies plus a note, dedupe deferred to a generate step.
- If the empirical test shows `SessionStart` stdout is dropped in `codex exec` (vs the TUI), the fallback is the static policy in `AGENTS.md` plus a context-loading skill; revisit the JSON `additionalContext` envelope only then.

## Alternatives considered

- Rewrite the scripts to emit Codex's JSON `additionalContext` envelope. Rejected: plain stdout already lands as context; a bespoke envelope adds drift for no confirmed gain.
- Symlink the scripts instead of copying. Rejected: a distributable bundle must be self-contained, and marketplace install and Windows do not handle symlinks reliably.

## References

- docs/specs/2026-07-13-0102-codex-plugin.md
- docs/research/2026-07-13-0102-codex-cli-extensibility.md
