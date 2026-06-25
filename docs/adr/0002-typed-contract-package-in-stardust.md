---
title: The typed contract package lives in stardust
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0144-jsonrpc-typed-contract.md
---

# The typed contract package lives in stardust

A shared Go package `stardust/rpc` defines the typed method params and results, and exo-jobs imports it.

## Context

The seam crosses the `map[string]any` boundary today. A typed contract needs one source of truth that both the server (stardust) and the client (exo-jobs) compile against. exo-jobs already imports `github.com/alxxpersonal/stardust` as a private module, so the access pattern exists.

## Decision

Define the contract as a package inside the stardust repo (`github.com/alxxpersonal/stardust/rpc`): per-method `Params` and `Result` structs, a typed `Record`, and a `ContractVersion` constant. exo-jobs imports it. No standalone module.

## Consequences

- The contract is compiler-checked on both sides; a renamed field fails to build, not at runtime.
- stardust remains the single source of truth for its own API.
- exo-jobs bumps its stardust dependency to current as part of the migration.
- No new repository or module to publish, version, and pin.

## Alternatives considered

- A separate shared module: adds a second versioned repo to publish and pin for no benefit, since stardust is already the import target.
- Generated types from a schema (proto, JSON Schema): adds a codegen step; hand-written typed structs are simpler at this scale.

## References

- exo-jobs `cli/src/go.mod`, `cli/src/internal/store/store.go`
- `sdk/client.go`
