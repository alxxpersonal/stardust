---
title: JSON-RPC 2.0 is the canonical contract transport
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0144-jsonrpc-typed-contract.md
---

# JSON-RPC 2.0 is the canonical contract transport

The stardust contract is JSON-RPC 2.0, not REST and not gRPC.

## Context

Stardust serves one core over two surfaces: a REST/JSON HTTP API and an MCP server. MCP is JSON-RPC 2.0. The operations are verb-shaped (`query`, `bundle`, `governs`, `remember`, `index`), which fit RPC method calls better than REST resources. The deployment is localhost, single-user, two Go binaries.

## Decision

Adopt JSON-RPC 2.0 as the one canonical contract. Serve it over stdio (MCP) and HTTP (`POST /rpc`). Keep a small number of plain-HTTP liveness endpoints (`/healthz`).

## Consequences

- One protocol spans the agent surface (MCP) and the programmatic surface (exo-jobs), so there is one mental model, one debugging story, and no second transport to maintain.
- The wire stays human-readable JSON, so `curl` and quick inspection survive.
- The existing REST contract in `docs/openapi.yaml` is superseded.
- No binary tooling (protoc, grpcurl) enters the build.

## Alternatives considered

- REST kept: leaves two drifting surfaces and is awkward for rich verb operations.
- gRPC + protobuf: binary wire kills curl-ability, needs HTTP/2 + protoc, does not speak MCP, and its perf and streaming wins are irrelevant on localhost.

## References

- JSON-RPC 2.0 specification
- `docs/openapi.yaml`, `internal/api/api.go`
