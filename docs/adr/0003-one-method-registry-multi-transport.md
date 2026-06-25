---
title: One method registry served over multiple transports
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0144-jsonrpc-typed-contract.md
---

# One method registry served over multiple transports

A single registry of typed JSON-RPC methods is dispatched by both the stdio (MCP) and HTTP transports.

## Context

The REST handlers in `internal/api/api.go` and the MCP handlers implement the same operations twice. Each new capability is wired in two places and can drift.

## Decision

Build one `map[string]Handler` keyed by JSON-RPC method name. Each handler decodes typed params, calls the existing `internal/service` method, and returns a typed result. Both transports are thin adapters over this one registry: the MCP stdio loop dispatches into it, and a single `POST /rpc` HTTP handler dispatches into it.

## Consequences

- A capability is registered once and is reachable from agents (MCP) and programmatic callers (HTTP) at the same time.
- The duplicated handler bodies are deleted.
- Transports become small and uniform; adding a transport later is an adapter, not a re-implementation.
- The MCP tool surface must be preserved or its plugin updated in lockstep.

## Alternatives considered

- Keep separate handler sets per transport: the current state, which drifts.
- A framework that owns routing: heavier than a `map[string]Handler` for this surface.

## References

- `internal/api/api.go`, the MCP server in `internal/`
