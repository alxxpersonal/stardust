---
title: Additive-then-retire migration
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0144-jsonrpc-typed-contract.md
---

# Additive-then-retire migration

The JSON-RPC registry is added alongside the existing REST, exo-jobs is migrated, then REST is retired. No flag day.

## Context

stardust and exo-jobs both run against real data. A single cutover that removes REST and switches the client at once risks a window where the server and client disagree.

## Decision

Migrate in ordered phases: (A) add the typed contract package, the registry, and the transports while REST stays live; (B) migrate exo-jobs' `store` to the typed client and bump the dependency; (C) retire the REST handlers and mark `openapi.yaml` superseded once nothing depends on them.

## Consequences

- The server keeps serving REST until the client no longer needs it, so there is no broken window.
- Each phase has its own build and test gate and can be merged independently.
- The contract `version` field plus a `status` handshake catch a stale client before it mismatches silently.
- A small amount of transient duplication exists during phase A and B, removed in phase C.

## Alternatives considered

- Big-bang cutover: simpler diff, but a real risk of a server/client mismatch window on live data.

## References

- docs/specs/2026-06-25-0144-jsonrpc-typed-contract.md
