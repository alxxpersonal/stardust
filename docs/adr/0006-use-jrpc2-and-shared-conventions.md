---
title: Use creachadair/jrpc2 and the shared JSON-RPC conventions
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0144-jsonrpc-typed-contract.md
  - docs/adr/0001-jsonrpc-canonical-transport.md
---

# Use creachadair/jrpc2 and the shared JSON-RPC conventions

stardust builds its JSON-RPC layer on `github.com/creachadair/jrpc2`, the same library and conventions the rest of the stack (exo-discord) uses, not a hand-rolled registry.

## Context

ADR 0001 made JSON-RPC the canonical transport. The first draft of the spec hand-rolled the dispatch (`map[string]Handler` + `encoding/json`). The exo-discord service already standardized on `creachadair/jrpc2` with a set of conventions. Two JSON-RPC implementations across one stack is the fragmentation this whole effort exists to remove.

## Decision

Use `creachadair/jrpc2`: a pure-Go, v1-stable, fully 2.0-complete library (batch, notifications, server push) with stdio and HTTP transports built in. Do NOT use the MCP go-sdk; it cages callers into tool/resource/prompt shapes.

Adopt the shared conventions:

- Architecture: MCP-aligned sibling, not raw MCP. Borrow the initialize handshake, capability negotiation, slash namespacing, and the isError-in-result split. A thin `mcp/` adapter at the edge provides MCP interop without the Tool/Resource cage.
- Methods: slash noun/verb (`record/get`, `record/create`, `records/list`, `query`, `index/run`).
- Transport: stdio with NDJSON first (all logs to stderr so a stray write cannot corrupt the stream), HTTP POST after.
- Docs: OpenRPC is the schema source of truth, with an `rpc.discover` introspection method. This replaces the REST `openapi.yaml`.
- Errors: standard JSON-RPC codes, the reserved `-32000..-32099` band for server and infrastructure errors, positive integers for domain errors (outside the reserved band).

## Consequences

- One JSON-RPC implementation and one mental model across stardust, exo-jobs, and exo-discord.
- jrpc2's `handler.Map` is the typed registry; less code than hand-rolling, with batching and notifications available without extra work.
- `rpc/contract.go` types feed jrpc2 handlers directly; the typed client wraps jrpc2's client.
- An OpenRPC document and `rpc.discover` supersede `docs/openapi.yaml`.
- One small pure-Go dependency is added; no codegen, no binary toolchain.

## Alternatives considered

- Hand-rolled registry: reimplements jrpc2's plumbing worse and drifts from the stack's one lib.
- MCP go-sdk: forces the tool/resource/prompt shape and is not a general JSON-RPC core.

## References

- `github.com/creachadair/jrpc2`
- OpenRPC specification
- exo-discord JSON-RPC architecture decisions
