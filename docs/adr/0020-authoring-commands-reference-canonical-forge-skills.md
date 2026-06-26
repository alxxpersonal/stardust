---
title: Plugin authoring commands reference the canonical forge skills
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-plugin-authoring-commands.md
  - docs/adr/0021-authoring-commands-delegate-never-reimplement.md
---

# Plugin authoring commands reference the canonical forge skills

The new `/stardust:spec`, `/stardust:plan`, `/stardust:doc`, and `/stardust:adr` commands name and route into the canonical spec-forge and doc-forge skills where they already live, rather than the plugin becoming their canonical home.

## Context

spec-forge, doc-forge, and stardust are canonical in forge-private and symlinked into the user skills dir (`~/.claude/skills/spec-forge -> /Users/alxx/Code/Self/forge-private/skills/spec-forge`, and likewise for doc-forge and stardust). There is exactly one copy of each skill on disk; any second copy is drift.

The plugin needs a write path to match its existing read path (the SessionStart injection of plans and specs). Two structures can supply it:

- (a) Reference: the plugin commands name and delegate to the canonical skills in forge-private. The plugin ships no copy.
- (b) Canonical home: move spec-forge, stardust, and doc-forge into `plugin/claude/skills/` and have forge-private symlink to the plugin.

## Decision

Take option (a). The authoring commands reference the canonical forge skills in place. The plugin tree contains no copy of spec-forge, doc-forge, or stardust. Option (b), making the plugin the canonical home with forge-private symlinking to it, is rejected as the default and is taken only if a deliberate canonical-home consolidation is later chosen and recorded in a superseding ADR.

## Consequences

- One copy of each skill stays on disk, so the write path and the read path share a single writing discipline and cannot drift.
- The plugin stays publishable without leaking the private forge skills into a public artifact.
- The full write workflow depends on spec-forge and doc-forge being installed. For a plugin consumer who lacks them, the commands MUST degrade to a documented docs-convention pointer rather than error.
- A future canonical-home consolidation remains possible but requires an explicit superseding decision, not an incidental file move.

## Alternatives considered

- Option (b), canonical home in the plugin: couples the private skill home to the plugin release cadence, risks shipping private skills publicly, and is a large restructure for no gain over referencing.
- Copy the skills into the plugin without symlinking: two divergent copies, the exact drift this setup exists to prevent.

## References

- docs/specs/2026-06-26-0418-plugin-authoring-commands.md
- ADR 0005 (Stardust manages docs in both repos): prior single-source-of-truth precedent
