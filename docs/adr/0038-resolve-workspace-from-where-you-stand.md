---
title: Resolve the workspace from where the session stands
status: Accepted
version: 1
date: 2026-07-02
related:
  - docs/adr/0037-per-project-vault-resolution.md
  - docs/specs/2026-07-02-0048-cwd-first-workspace-resolution.md
  - plugin/claude/scripts/resolve-root.sh
---

The plugin resolves the workspace by walking up from the current directory, git-style, with an env override first and the per-project vault map as the fallback.

## Context

`resolve-root.sh` keyed repo mode on one exact check: `$CLAUDE_PROJECT_DIR/.stardust`. Two failures follow. First, `CLAUDE_PROJECT_DIR` is not present in the Bash tool environment where slash commands actually run the resolver (verified: unset in live sessions), so repo mode never fired even when the shell stood inside an initialized workspace. Second, there was no walk-up, so a session in a subdirectory of a workspace, or in a git worktree checkout beside it, resolved to none. The observed symptom: a directory with `.stardust/` in plain sight and the resolver still printing `MODE=none`, which starves those sessions of the injected workspace state.

## Decision

Resolution is layered, most local truth first:

1. `STARDUST_VAULT`, when set to an existing directory: the explicit override always wins.
2. Walk up from `$PWD` to `/`; the first directory containing `.stardust/` resolves as repo mode. This is how git resolves its root, and it makes the answer follow where the session actually stands.
3. Walk up from `$CLAUDE_PROJECT_DIR` the same way, when it is set: a session that started in a workspace keeps resolving it after the shell moves elsewhere.
4. The per-project `vaults` map (ADR 0037): exact key for `$CLAUDE_PROJECT_DIR` then `$PWD`, then the longest key that is a path prefix of either, so subdirectories of a mapped project inherit its vault.
5. The legacy top-level `vaultPath`, only when no `vaults` map exists.
6. Otherwise none.

The script emits a third line, `SOURCE=<env|cwd|project|vault-map|legacy|none>`, so a misresolution is diagnosable in one glance. The `MODE`/`ROOT` contract is unchanged and consumers need no edits.

## Consequences

- A session standing in, or below, an initialized workspace always resolves it, regardless of whether Claude Code populated `CLAUDE_PROJECT_DIR`.
- Worktree and subdirectory sessions resolve their own checkout rather than none.
- A session that wandered off with `cd` still resolves its home workspace through layer 3, so injected stardust context keeps flowing.
- Precedence is deterministic when layers disagree: the shell's own position wins over the session's start dir, which wins over configured maps.
- The isolation property of ADR 0037 is preserved: an unmapped, un-walkable directory resolves to none, never to another project's vault.

## Alternatives considered

- Fix only the env var (read `$PWD` when `CLAUDE_PROJECT_DIR` is unset) without walking up. Rejected: leaves subdirectory and worktree sessions broken.
- Walk up for vault mode too (auto-detect Obsidian vaults). Rejected: vault mode exists to use a configured vault from outside it; `.stardust/` presence already covers initialized vaults via repo mode.
- Cache the resolution in plugin data. Rejected: stale-cache bugs are the disease this ADR cures; the walk is a handful of stat calls.

## References

- plugin/claude/scripts/resolve-root.sh
- docs/adr/0037-per-project-vault-resolution.md
