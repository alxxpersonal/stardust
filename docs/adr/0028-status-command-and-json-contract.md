---
title: Status command gathers state in the service layer with an ANSI-free JSON contract
type: adr
status: Accepted
created: 2026-06-26
updated: 2026-07-03
related:
  - internal/cli/status.go
  - internal/service/service.go
  - internal/service/records.go
  - internal/service/bundle.go
  - docs/specs/2026-06-26-2104-init-detect-and-status.md
---

# 0028. Status command gathers state in the service layer with an ANSI-free JSON contract

## Context

There is no `stardust status`. Checking whether a directory is initialized, what kind it is, which collections exist with what counts, whether vectors are live, and how far behind HEAD the index sits, requires reading files and running several subcommands. Agents and the SDK have no single parseable state probe. The existing `service.Status` reports index health only, is a method on an already-open `Service`, and is consumed by the `rpc`/`mcp` "status" handler with a fixed contract, so it cannot express the uninitialized case nor carry collections and kind.

## Decision

Add a package-level `service.GatherStatus(ctx, start) (VaultStatus, error)` that resolves the root from `start` via `config.FindRoot` and reports full state. When no `.stardust` is found it returns an uninitialized `VaultStatus` (detected kind plus an init hint) and a nil error, so "not initialized" is a normal result, not an error. When found, it opens the service and composes the existing reads: `svc.Status` (index health), `svc.ListCollections` (collections with live counts), `svc.commitsBehindHead` (freshness), and `convention.DetectKind` (kind). The vectors-off reason reuses the existing `ftsOnlyReason` constant so the explanation is identical across surfaces.

A new thin `cli/status.go` command resolves `start` from `STARDUST_VAULT` or cwd (same precedence as `openService`), calls `GatherStatus`, and renders. `--output json` writes indented JSON straight through `emitJSON` on `cmd.OutOrStdout`, never through fang, matching the query/bundle/registry discipline so piped JSON carries zero ANSI bytes. The default is a compact human-readable block. `VaultStatus` and `IndexHealth` carry `json` struct tags on every field; `Collections` is always a non-nil slice so the JSON array is `[]`, never `null`.

`GatherStatus` is package-level rather than a `Service` method because the uninitialized case must report before any `Service` can be opened. `service.Status` and the `rpc`/`mcp` "status" handler are left untouched.

## Consequences

- One call returns the whole state probe for humans (compact block) and agents (ANSI-free JSON), with the data gathering in the service layer and the CLI thin.
- Reuses `Status`, `ListCollections`, `commitsBehindHead`, `DetectKind`, and `ftsOnlyReason` rather than re-deriving any of them, so the report cannot drift from the canonical reads.
- `service` gains a `convention` import; verified acyclic (`convention` does not import `service`).
- `status` probes Ollama via `Status` (`embed.Available`), adding a network touch and latency; accepted as the honest vectors signal, consistent with existing `Status` behavior.

## Alternatives considered

- Extend `service.Status` instead of adding `GatherStatus`. Rejected: `Status` has a fixed `rpc`/`mcp` contract and is a method on an open `Service`, so it cannot express the uninitialized case.
- Put data gathering in the CLI. Rejected: violates the thin-CLI, service-parity principle; every surface must reach the same capability.
- Expose `GatherStatus` over `rpc`/`mcp`/HTTP now. Deferred; structurally a thin later surface per the SPEC parity note.

## References

- `docs/specs/2026-06-26-2104-init-detect-and-status.md`
- `internal/service/service.go`, `internal/service/records.go`, `internal/service/bundle.go`
- `internal/cli/output.go`, `internal/cli/headless_ansi_test.go`
