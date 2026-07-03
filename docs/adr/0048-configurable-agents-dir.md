---
title: The agent assets home is configurable with a docs/agents default
status: Accepted
version: 1
date: 2026-07-03
related:
  - docs/specs/2026-07-03-1456-configurable-agents-dir.md
  - docs/adr/0047-agent-assets-docs-home.md
---

One `agents_dir` key in `.stardust/config.toml` relocates the agent-assets home; absent means `docs/agents`, and every convention seam follows the configured value.

## Context

ADR 0047 fixed the agent-assets home at `docs/agents/` across six seams (sync source defaults, stray-doc and orphan exemptions, the registry Agents section, the init scaffold). Some repos want the area elsewhere (`docs/skills/`, a non-docs dir). sync.toml can already override the sync sources, but the convention layer kept enforcing the hardcoded path, so a relocated home was flagged stray, unlisted, and never scaffolded.

## Decision

- `internal/config.Config` gains `AgentsDir` (toml `agents_dir`), repo-relative, empty means `docs/agents`. A single resolver method owns the default and normalization; no seam hardcodes the literal anymore.
- All six seams read the resolved value. Kind subfolders (`skills/`, `subagents/`, `rules.md`) keep fixed names inside it.
- `stardust check` validates the knob (`bad-agents-dir`): absolute paths, repo-escaping paths, and collisions with registered collection folders are config errors.
- `sync.toml` explicit sources remain the stronger override, unchanged.

## Consequences

- Existing workspaces are byte-identical: no knob, same default, same behavior.
- A repo can home its assets anywhere repo-relative; outside `docs/` it simply opts out of the docs-tree exemptions it no longer needs, while index, search, sync, and the registry section still work.
- Pointing the knob away from a populated `docs/agents/` makes the old copies ordinary docs content again, and stray-doc flags them, which is the correct finish-the-move signal.
- `agentsync.DefaultConfig` and `convention.CheckDocs` gain the dir as an explicit input; two production call sites each, no global state.

## Alternatives considered

- Deriving the home from sync.toml: the convention seams must not depend on the sync layer's config, and most repos have no sync.toml. Rejected.
- Per-kind path configuration: schema sprawl with no asked-for benefit. Rejected.
- An environment variable: the home is a repo property, not a session property. Rejected.

## References

- docs/specs/2026-07-03-1456-configurable-agents-dir.md
- internal/config/config.go, internal/agentsync/config.go, internal/convention/check.go, internal/service/check.go, internal/service/registry.go, internal/cli/init.go

- Reviewed 2026-07-03 after implementing Task 1: the referenced seams (agentsync sources, check exemptions, registry section, init scaffold) now read the resolved agents dir; the decision is realized in code, references hold.
