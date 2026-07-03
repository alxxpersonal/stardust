---
title: Agent assets live in docs/agents and sync automatically
status: Accepted
version: 1
date: 2026-07-03
related:
  - docs/specs/2026-07-03-0523-agent-assets-docs-home.md
  - docs/adr/0039-rules-adapter-sync.md
---

`docs/agents/` is the one canonical home for shared agent assets, the convention knows the area natively, placement alone decides sharing, and the git hooks plus a CI check make sync automatic.

## Context

agentsync's canonical sources are scattered outside the doc workflow: repo-root `skills/` and `agents/` plus `.stardust/rules.md`. Shared agent assets are therefore invisible to index, search, registry, and drift, sharing intent is not readable from the tree, and sync only happens when someone remembers to run it. The user set three requirements: a docs-homed canonical source with kind subfolders stardust understands natively, location-based sharing opt-in, and automatic sync with a CI gate.

## Decision

1. **Home**: `docs/agents/skills/<name>/SKILL.md`, `docs/agents/subagents/<name>.md`, `docs/agents/rules.md`. `stardust init --docs` scaffolds the area.
2. **Native**: `docs/agents/` is a convention-known area beside the collections (the `docs/templates/` pattern), never a stray-doc, listed in the registry's Agents section, validated by the skill and agent checks, indexed and drift-tracked like any note. It is NOT a registered collection, because skill frontmatter is not doc frontmatter.
3. **Sharing by placement**: under `docs/agents/` means synced to every configured tool; directly in a tool dir means tool-local, never touched. `targets:` frontmatter remains an optional narrowing. Root `skills/`, `agents/`, and `.stardust/rules.md` demote to ImportOnly compat.
4. **Auto-sync**: the guarded hook bodies (post-commit, post-merge, owned and composed alike) gain a `stardust sync` line, and CI runs `stardust sync --check` as a drift gate. Hooks materialize symlinks on developer machines; CI verifies and cannot materialize, and the docs say so plainly.

## Consequences

- One glance at `docs/agents/` shows the entire cross-agent surface, and it rides every doc workflow: search, registry, check, drift.
- Adding a skill is one file in one place; every configured harness gets it on the next commit or pull with no manual step.
- Existing repos keep working through compat sources and can migrate by moving files.
- Commits gain a no-op-fast sync pass; the line is guarded and can never fail a commit.
- Deferred, with revisit triggers: KindCommand (harness command formats diverge: markdown frontmatter for claude, plain prompt files for codex, TOML for gemini; translation is lossy today, revisit when two harnesses converge), MCP config sync (server declarations embed tool-specific transport and permission shapes), `.agents/` as a target (no confirmed native reader; it stays ImportOnly).

## Alternatives considered

- A registered `agents` collection: wrong validation regime for skill frontmatter and per-repo ceremony.
- Keeping root `skills/` and `agents/` canonical: rejected by the user; assets belong in the doc workflow.
- Frontmatter opt-in for sharing: placement is simpler, visible, and ceremony-free.
- CI-side symlink creation: mechanism confusion; CI verifies, hooks materialize.

## References

- docs/specs/2026-07-03-0523-agent-assets-docs-home.md
- internal/agentsync/config.go, internal/convention/check.go, internal/hooks/hooks.go
