---
title: Cwd-first workspace resolution - implementation plan
status: Done
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-0048-cwd-first-workspace-resolution.md
  - docs/adr/0038-resolve-workspace-from-where-you-stand.md
---

Rewrite the plugin resolver to walk up from the cwd with an env override and the vault map as fallback, pin every layer with a test script, and sync the live plugin cache.

## Header

- **Goal:** sessions resolve their workspace from where they stand; `MODE=none` only when nothing is genuinely there.
- **Architecture:** one POSIX script, six resolution layers, first hit wins, plus a `SOURCE=` diagnosis line. Contract (`MODE`/`ROOT` lines) unchanged.
- **Tech stack:** POSIX sh, jq for the map lookups. No new dependencies.
- **Global constraints:** conventional commits, no co-author trailers, zero em or en dashes, `sh -n` clean, the test script green before every commit.

## Context

Read first: `plugin/claude/scripts/resolve-root.sh` (the current single-check resolver), `plugin/claude/commands/setup.md` (documents resolution for users), `plugin/claude/hooks/session-start.sh` (the consumer contract), ADRs 0037 and 0038, and the spec's Approach table (the six layers and SOURCE tags are normative).

## Task 1: layered resolver + pinned tests

Files:

- Modify: `plugin/claude/scripts/resolve-root.sh`
- Create: `plugin/claude/scripts/resolve-root.test.sh`

Steps:

- [x] Write `resolve-root.test.sh` first: POSIX sh, self-contained temp dirs, one function per spec verification case (all ten), asserting `MODE`, `ROOT`, and `SOURCE`; nonzero exit on any failure. Run it against the current resolver and confirm the walk-up and env cases fail.
- [x] Rewrite `resolve-root.sh`: keep `emit` eval-safe quoting, add the `SOURCE=` third line, implement layers in order: `STARDUST_VAULT` override, physical walk-up from `$PWD`, walk-up from `$CLAUDE_PROJECT_DIR`, `vaults` exact then longest-prefix lookup (jq, keys as path prefixes of `$PWD` and `$CLAUDE_PROJECT_DIR`), legacy `vaultPath` only when no `vaults` key, then none. Mode gate for vault layers stays `vault|auto`.
- [x] Run the test script; loop to green. `sh -n` both scripts.
- [x] Commit `fix(plugin): resolve the workspace from the cwd with walk-up and env override`.

## Task 2: docs, cache sync, index

Files:

- Modify: `plugin/claude/commands/setup.md`
- Sync: the resolver, test script, and setup.md into `~/.claude/plugins/cache/stardust-local/stardust/0.5.0/`
- Modify: `docs/INDEX.md` (regenerated)

Steps:

- [x] Update setup.md's mode/resolution prose to the six layers (tight, table form), including `STARDUST_VAULT` and the `SOURCE=` line.
- [x] Copy `scripts/resolve-root.sh`, `scripts/resolve-root.test.sh`, and `commands/setup.md` into the active 0.5.0 plugin cache; run the test script against the cache copy too.
- [x] Run `stardust index` and `stardust registry` from the repo root.
- [x] Dash-scan every touched file (no U+2014, no U+2013).
- [x] Commit `docs(plugin): document layered workspace resolution`.

## Task 3: adversarial review

Steps:

- [ ] Fresh shell: run `resolve-root.test.sh` end to end; all ten cases green.
- [ ] Reproduce both original failures against the NEW resolver: (a) cd into a dir containing `.stardust/` with `CLAUDE_PROJECT_DIR` unset resolves repo with SOURCE=cwd; (b) a nested subdirectory resolves the same root.
- [ ] Prove ADR 0037 isolation still holds: an unmapped temp dir with nothing to walk to resolves none, and two mapped projects resolve two different vaults.
- [ ] Verify the live cache copy matches the repo copy byte for byte, and `git log` shows clean conventional commits with no trailers.
- [ ] Report defects; do not fix silently.

## Verification

The spec's ten pinned cases green in the repo AND the live cache; both original failure repros now resolve; isolation preserved; `MODE`/`ROOT` contract byte-compatible; dash-clean.

## Self-review gate

- Every spec goal maps to a test case; every layer has a SOURCE tag asserted.
- No consumer (hooks, commands) required an edit.
- The resolver stays always-exit-0 and silent on stderr in normal paths.
