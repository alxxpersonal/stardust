---
title: Rules sync composes a canonical source through per-tool adapters
status: Accepted
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-0316-rules-adapter-sync.md
  - docs/plans/2026-07-02-0316-rules-adapter-sync.md
  - docs/adr/0007-stardust-composes-hooks-never-clobbers.md
  - docs/adr/0008-sentinel-block-hook-injection.md
  - internal/agentsync/inventory.go
  - internal/agentsync/plan.go
  - internal/hooks/block.go
---

# Rules sync composes a canonical source through per-tool adapters

Rules are authored once in a canonical `.stardust/rules.md`, and `stardust sync` renders that body per tool and composes it into a sentinel-delimited block inside `CLAUDE.md`, `AGENTS.md`, and `GEMINI.md`, never a symlink and never clobbering user lines.

## Context

SPEC section 4.2 and the README both defer rules-adapter sync: skills and agents sync today, but `CLAUDE.md` / `AGENTS.md` / `GEMINI.md` "need format-aware adapters, not blind symlinks." Three constraints make this different from the skill and agent path already in `internal/agentsync`:

1. Rules are one logical body per workspace, not many items discovered by walking a folder.
2. The targets are single files at the repo root that a human and other tools also write, not tool-owned asset directories. A symlink or byte copy would either clobber that file or point the whole file at stardust.
3. Each tool consumes its memory file slightly differently, so the render path has to be per tool even when the bodies currently converge.

The hooks subsystem already solved the "own part of a shared file" problem: ADR 0007 (compose, never clobber) and ADR 0008 (sentinel-block idempotent injection). Rules sync is the same shape applied to markdown memory files instead of shell hook files.

## Decision

Extend `internal/agentsync`, do not build a parallel system.

- **Canonical source.** A single hand-authored `.stardust/rules.md` (new `Layout.Rules()`), committed, sitting beside `sync.toml` and `config.toml`. Optional YAML frontmatter (`name`, `targets: [...]`) is honored exactly as for skills and agents.
- **New kind.** `KindRules` joins `KindSkill` and `KindAgent`. A `rules` source resolves to that one file and yields exactly one `Item`. Config gains a `rules` source kind and a `RulesPath` field on `Target`, mirroring `SkillsPath` / `AgentsPath`. `DefaultConfig` wires the repo-scope targets: claude to `CLAUDE.md`, codex to `AGENTS.md`, gemini to `GEMINI.md`; empty `RulesPath` targets are skipped.
- **Sentinel-block compose.** A markdown-flavored mirror of `internal/hooks/block.go`: HTML-comment markers `<!-- >>> stardust rules >>> (managed block, do not edit) -->` and `<!-- <<< stardust rules <<< -->` so they stay invisible when rendered. Injection is a single read-modify-write that replaces the block in place (idempotent) and never touches lines outside the markers. Missing files are created.
- **Format adapters.** A per-tool adapter map keyed by `Tool` renders the canonical body into the block each target expects (strip source frontmatter, normalize, apply any tool wrapper). Today the three share one markdown renderer; the map gives each tool its own seam so a later divergence (Gemini `@import`, Claude `@path` includes, AGENTS.md conventions) lands for one tool without touching the others.
- **One command, one config.** Rides `stardust sync`. Rules whose block is absent count as missing, stale blocks count as drift, so `stardust sync --check` already exits non-zero and `stardust sync` writes. No new command, no new flag, no new config file.

## Consequences

- `stardust sync` now maintains rules, closing the deferred item in SPEC 4.2 and the README.
- A user editing `CLAUDE.md` outside the block is safe: re-sync replaces only the managed block, exactly as hooks do.
- Rules drift is repaired by a plain `stardust sync`, without `--repair`. Because stardust owns only its sentinel block (ADR 0007), re-composing is non-destructive, unlike a symlink whose drift may encode user intent. The `--repair` guard stays meaningful for `symlink` and `copy` targets; `compose` targets self-heal. This asymmetry is deliberate and keys off the action mode.
- Rules never report a `conflict`. A file that exists without our block is composed into, not refused, because we never claim the whole file.
- The adapter seam is real even while bodies converge, so "format-aware" is structural, not cosmetic.

## Alternatives considered

- **Symlink `CLAUDE.md` to `.stardust/rules.md`.** The rejected baseline. It seizes the whole file, breaks the moment a human or another tool writes non-rules content there, and ignores per-tool format needs.
- **Byte-copy the canonical file to each target.** Same clobber problem, plus no place for per-tool rendering.
- **A separate `stardust rules` command and a `rules.toml`.** New surface for no gain. Discovery, planning, `--check`, and `--repair` already exist on `stardust sync`; rules is another kind, not another subsystem.
- **Reuse `internal/hooks/block.go` directly.** Its markers are shell comments and its helpers are unexported. Extracting a shared block package is a refactor of working hook code with no caller yet; mirroring the small pattern in `agentsync` with markdown markers is lower risk. Revisit if a third consumer appears.

## References

- SPEC.md section 4.2, README rules-adapter-sync deferral
- docs/adr/0007-stardust-composes-hooks-never-clobbers.md
- docs/adr/0008-sentinel-block-hook-injection.md
- `internal/hooks/block.go`
- `internal/agentsync/inventory.go`, `internal/agentsync/plan.go`, `internal/agentsync/apply.go`
