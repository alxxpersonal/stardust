---
title: Per-project vault resolution
status: Accepted
version: 1
date: 2026-06-30
related:
  - docs/specs/2026-06-30-0417-vault-resolution-isolation.md
  - plugin/claude/scripts/resolve-root.sh
---

The stardust plugin resolves the vault per project directory, not from one global path.

## Context

`resolve-root.sh` resolved vault mode from a single `vaultPath` in the one global `config.json` under `CLAUDE_PLUGIN_DATA`. That data dir is shared by every Claude session, so every session that did not resolve a repo (`$CLAUDE_PROJECT_DIR/.stardust`) read the same `vaultPath`. Concurrent sessions in different roots collided on one vault, and any `/stardust:setup` overwrote it for all of them.

## Decision

Vault mode resolves the vault from a `vaults` map in `config.json`, keyed by `CLAUDE_PROJECT_DIR`. Each session reads only its own entry, so sessions never share a global path. A legacy top-level `vaultPath` is read only when the config has no `vaults` map yet, so pre-migration configs keep working until they are migrated. `/stardust:setup` merges a per-project entry and drops `vaultPath`.

## Consequences

- Concurrent sessions in different project dirs resolve independent vaults; the collision is gone.
- A project dir with no mapping resolves to `none` rather than a stale global vault. The user runs `/stardust:setup` in that dir to map it.
- Repo mode is unchanged: a `$CLAUDE_PROJECT_DIR/.stardust` still wins and is already per-project.
- The `MODE` and `ROOT` output contract is unchanged, so hooks and commands need no change.

## Alternatives considered

- Keep one global `vaultPath` with per-project overrides layered on top. Rejected: the global default reintroduces the collision for any unmapped dir.
- Walk up from the cwd to auto-detect a vault. Rejected: vault mode exists precisely to use a configured vault from outside it, so walk-up defeats the purpose.

## References

- plugin/claude/scripts/resolve-root.sh
- plugin/claude/commands/setup.md
- docs/specs/2026-06-30-0417-vault-resolution-isolation.md
