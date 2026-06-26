---
title: Authoring commands delegate to skills and never reimplement the doc convention
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-plugin-authoring-commands.md
  - docs/adr/0020-authoring-commands-reference-canonical-forge-skills.md
---

# Authoring commands delegate to skills and never reimplement the doc convention

Each authoring command resolves workspace state and surfaces context, then hands off to the named canonical skill. It holds no embedded copy of the frontmatter, section layout, collapsible markup, ADR numbering, or index regeneration.

## Context

A slash command runs as a prompt under the tool whitelist in its `allowed-tools` frontmatter. The Skill tool is not granted to the existing stardust commands, and the established pattern (status, refresh, crons) is a command that resolves state and stops, not one that performs a deep workflow inline.

The tempting shortcut is to have a command write the doc itself with the Write tool, embedding the doc convention in the command body. That forks the convention into the command and drifts from spec-forge and doc-forge the moment either skill changes a field or a section.

## Decision

The four authoring commands are thin routers. Their `allowed-tools` is `Bash, Read` only, with no `Write`. The command turn performs precondition checks (resolve-root), argument parsing and validation, and read-only surfacing (recent specs, active plans, next ADR number), then delegates to the canonical skill.

The normative delegation mechanism is a terminal handoff: the command's final output is the exact skill invocation to run next (for example `/spec-forge "<topic>"` or `/doc-forge adr "<decision>"`). If a future harness exposes the Skill tool to commands, a command MAY add `Skill` to `allowed-tools` and invoke the canonical skill in the same turn; the handoff remains the fallback. A command MUST NOT reproduce the skill's writing steps inline.

## Consequences

- One writing discipline, in the skill. The commands cannot drift from it because they do not contain it.
- Omitting `Write` from `allowed-tools` makes any future inline-authoring change visible and reviewable rather than silent.
- The full workflow (exploration, validation, index regeneration, native-plan sync) runs with the skill's complete discipline, not a thinned command copy.
- The command turn never writes a file, so `git status` is unchanged by running a command; only the delegated skill writes.

## Alternatives considered

- Thick commands that write docs directly: forks the convention into four command bodies; rejected.
- Granting `Write` to the commands as a convenience: invites the same fork; rejected.

## References

- docs/specs/2026-06-26-0418-plugin-authoring-commands.md
- spec-forge SKILL.md, doc-forge SKILL.md
- plugin/claude/commands/status.md, refresh.md, crons.md
