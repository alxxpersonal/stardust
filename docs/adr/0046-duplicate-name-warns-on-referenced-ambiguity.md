---
title: Duplicate-name warns only on referenced ambiguity
status: Accepted
version: 1
date: 2026-07-03
related:
  - internal/service/check.go
  - docs/adr/0037-directory-index-convention.md
---

The duplicate-name check warns on a shared bare note name only when a wikilink actually targets it; collection-qualified slug reuse keeps warning unconditionally.

## Context

`stardust check` warned whenever two files shared a normalized note name. For docs collections that is a genuine convention violation (one slug, one doc). For free-form notes it flagged ordinary repo shape forever: `README.md` beside `plugin/claude/README.md` and `plugin/obsidian/README.md`, or `SPEC.md` beside the `/stardust:spec` command file. Those names cannot be renamed (README is a platform convention, a command file's name defines its slash command), cannot be ignored (the ignore mechanism is directory-scoped by design), and the files must stay indexed. The repo could therefore never reach 0 warnings, and a permanent warning trains the reader to ignore the surface, the same failure mode drift review-prompts (ADR 0018) and contradiction candidates (ADR 0043) are designed around.

## Decision

The warning's own rationale is "wikilinks to it are ambiguous", so the check now enforces exactly that:

- A bare basename shared by free-form notes warns only when some note's wikilink targets that name. The target set is gathered lazily, only when a bare duplicate exists.
- A collection-qualified duplicate (slug reuse inside one `docs/<collection>/`) always warns; the docs naming convention genuinely forbids it.

## Consequences

- Multi-README repos and command-file name collisions are silent until a `[[readme]]`-style link makes the ambiguity real, at which point the warning fires with the same detail text.
- The repo reaches `stardust check` 0 errors, 0 warnings honestly, without renames, ignores, or checker gutting.
- A vault that links by a shared bare name still gets warned, so the check's protective purpose is intact; the change narrows when it speaks, not what it protects.
- Pinned by TestCheckBareDuplicateNameWarnsOnlyWhenReferenced (silent unreferenced twins, warns once referenced) alongside the existing cross-collection and in-collection tests, which are unchanged.

## Alternatives considered

- Renaming the colliding files: README.md is a platform convention and the command file's name is its slash command. Rejected.
- A file-level ignore mechanism: adds config surface to hide a symptom, and the files must remain indexed and linkable. Rejected.
- Keying duplicate-name by the folder-aware GraphKey: contradicts the test-locked CollectionKey design in which bare names are the wikilink resolution space. Rejected.
- Accepting a permanent 2-warning floor: normalizes a dirty baseline and defeats the zero-warning ratchet. Rejected.

## References

- internal/service/check.go (the gate and wikilinkTargets)
- internal/service/check_test.go (the pin)
- docs/adr/0018-drift-detection-by-commit-distance.md (review-prompt precedent)
