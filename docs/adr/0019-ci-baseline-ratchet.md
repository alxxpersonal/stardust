---
title: CI adopts via a baseline and ratchets on new errors only
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-doc-code-coherence-engine.md
---

# CI adopts via a baseline and ratchets on new errors only

`stardust check --ci` reports and fails only on issues absent from a committed baseline, so a dirty repo adopts green and the gate catches new rot.

## Context

`stardust check --strict` returns a total error count (`check.go`), so a repo with an existing backlog (the current tree has on the order of 81 issues) cannot adopt the gate without first fixing everything, which never happens. The result is that CI enforcement is all-or-nothing and therefore off. A ratchet is the standard fix: freeze the known backlog, fail only on new entries.

## Decision

Add a committed baseline file `.stardust/baseline.json` storing a stable fingerprint per known issue: the tuple of kind, path, and a normalized detail. `stardust check --ci` computes the current issue set, subtracts the baseline, reports only the difference, and exits non-zero solely when new issues exist. `stardust check --update-baseline` snapshots the current set after an intentional change. The fingerprint is content-derived, so an issue that is fixed and later reintroduced re-fires rather than staying masked.

## Consequences

- A repo with a backlog adopts the gate immediately and green; the wall is gone.
- New rot fails the PR while the backlog is burned down separately.
- A baseline can hide regressions if never refreshed; mitigated because the fingerprint is content-derived and `--update-baseline` is explicit.
- The baseline is committed, so its diff is reviewable and its growth is visible in PRs.

## Alternatives considered

- Fail on total count over a threshold. Rejected: thresholds drift and do not distinguish new from old.
- Store the baseline in config. Rejected: a standalone JSON keeps the issue list out of human-edited config and makes its diff legible.
- Suppress via inline doc annotations. Rejected: scatters suppression across files and is easy to abuse; a central baseline is auditable.

## References

- `internal/convention/check.go`, `internal/service` check surface, `internal/cli` check command.
- ADR 0018 (drift warnings the ratchet can later gate on).
