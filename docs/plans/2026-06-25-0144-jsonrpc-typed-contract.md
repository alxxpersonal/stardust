---
title: JSON-RPC typed contract rebuild - implementation plan
status: Draft
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0144-jsonrpc-typed-contract.md
---

# JSON-RPC typed contract rebuild - implementation plan

Build the typed JSON-RPC registry in stardust, migrate exo-jobs onto the typed client, retire REST, and set up `.stardust` docs in both repos.

## Header

- **Goal:** one typed JSON-RPC 2.0 method registry as the canonical stardust contract, served over stdio (MCP) and HTTP, shared as a Go package both binaries import, with `map[string]any` removed from the exo-jobs seam.
- **Architecture:** `stardust/rpc` typed contract package -> a jrpc2 `handler.Map` over `internal/service` -> jrpc2 stdio + `jhttp` (`POST /rpc`) transports, plus a thin `mcp/` edge adapter for MCP interop; exo-jobs `store` consumes the typed `rpc.Client` (jrpc2 client).
- **Tech stack:** Go 1.26, `github.com/creachadair/jrpc2` (server `handler.Map`, `jhttp` HTTP bridge, stdio channel, typed client), the existing `internal/service` core. One small pure-Go dependency (ADR 0006), no codegen.
- **Global constraints:** conventional commits, no co-author or generated trailers, gofmt clean, `go test ./...` green and `make lint` clean before every commit in each repo, ZERO em dashes or en dashes, no emoji.

## Context

Stardust runs one service core behind duplicated REST and MCP handlers. exo-jobs consumes REST through `sdk.Client` (six methods) and pins a stale stardust version. The spec (`docs/specs/2026-06-25-0144-jsonrpc-typed-contract.md`) and ADRs 0001 to 0005 lock the design. This plan is additive-then-retire (ADR 0004).

## Reuse map (read first)

- stardust `internal/api/api.go` - the 21 REST routes + their handler bodies to mirror into the registry.
- stardust `sdk/client.go` - the 16 client methods + their current JSON shapes; the typed `Client` supersedes this.
- stardust `internal/service/*.go` - the real method signatures the handlers call. Confirm each in source.
- stardust the MCP server (grep `internal/` for `--mcp` / the JSON-RPC stdio loop) - the dispatch point to reuse.
- exo-jobs `cli/src/internal/store/store.go` - the six `sdk.Client` calls + the `map[string]any` record mapping to retype.
- exo-jobs `cli/src/go.mod` - the stardust dependency to bump.

The executor MUST mirror these tasks into the harness todo tool, keep exactly one task in progress, and tick each box live. Do not exit a task until its validation loop is green.

## Phase A - stardust contract, registry, transports

### Task A1: typed contract package skeleton

- Create: `rpc/contract.go`, `rpc/record.go`
- Test: `rpc/contract_test.go`
- Produces: `const ContractVersion`, typed `Record`, and `Params`/`Result` structs for the six seam methods first (`record/create`, `record/get`, `record/list`, `record/patch`, `record/delete`, `status`).

- [ ] Write `rpc/contract_test.go` asserting each `Params`/`Result` round-trips through `json.Marshal`/`Unmarshal` with stable field names.
- [ ] Run `go test ./rpc/` and confirm it fails to compile (types undefined).
- [ ] Define `Record` and the six method `Params`/`Result` in source, field names matching the current JSON in `sdk/client.go` (confirm each there).
- [ ] Run `go test ./rpc/`; loop until green.
- [ ] `go build ./...`, `gofmt -l .`, `make lint`; commit `feat(rpc): add typed contract package for the record seam`.

### Task A2: jrpc2 method registry over the service core

- Create: `internal/rpcserver/registry.go`
- Test: `internal/rpcserver/registry_test.go`
- Consumes: `internal/service` methods, `rpc` types, `github.com/creachadair/jrpc2/handler`.
- Produces: a `handler.Map` of the slash method names to typed handlers (each `func(ctx, rpc.XParams) (rpc.XResult, error)`), assembled by `NewRegistry(svc *service.Service) handler.Map`.

- [ ] Write a test that builds the registry and calls the `record/create` handler with typed params via `jrpc2.NewLocal`, asserting a typed `Record` back.
- [ ] Run it, confirm failure.
- [ ] Implement `NewRegistry` and the six typed handlers, each calling the confirmed `internal/service` method. jrpc2 owns decode, encode, ids, and the error band (ADR 0006).
- [ ] Run the test; loop to green.
- [ ] Gate, commit `feat(rpcserver): add jrpc2 method registry`.

### Task A3: HTTP /rpc adapter (jhttp)

- Modify: `internal/api/api.go`
- Test: `internal/api/rpc_test.go`
- Consumes: the registry, `github.com/creachadair/jrpc2/jhttp`.

- [ ] Write a test that posts a JSON-RPC 2.0 envelope to `/rpc` and asserts a JSON-RPC result for `status`.
- [ ] Run it, confirm failure.
- [ ] Mount a `jhttp.Bridge` over the registry at `POST /rpc`. Leave the REST routes in place (additive); keep `GET /healthz` plain.
- [ ] Run the test; loop to green.
- [ ] Gate, commit `feat(api): serve the jrpc2 registry over POST /rpc`.

### Task A4: thin mcp adapter over the registry

- Create: `internal/mcp/adapter.go` (or modify the existing MCP server file from the reuse map)
- Test: the MCP test file, extended.

- [ ] Write a test asserting an MCP call for a registry method resolves through the shared registry and preserves the existing MCP tool name.
- [ ] Run it, confirm failure.
- [ ] Implement a thin `mcp/` adapter mapping MCP framing (initialize handshake, capability negotiation, isError-in-result) onto the same jrpc2 registry, preserving the current MCP tool names and schemas. Do not adopt the MCP go-sdk (ADR 0006).
- [ ] Run the test; loop to green. Manually confirm the MCP plugin tool list is unchanged.
- [ ] Gate, commit `refactor(mcp): map mcp framing onto the jrpc2 registry`.

### Task A5: typed client

- Create: `rpc/client.go`
- Test: `rpc/client_test.go` (httptest server)

- [ ] Write a test that stands an httptest `/rpc` server and asserts `Client.RecordGet` (calling `record/get`) returns a typed `Record`.
- [ ] Run it, confirm failure.
- [ ] Implement `rpc.Client` wrapping jrpc2's client (`jhttp.NewChannel` + `jrpc2.NewClient`), with typed methods for the six seam operations.
- [ ] Run the test; loop to green.
- [ ] Gate, commit `feat(rpc): add typed jrpc2 client`.

## Phase B - migrate exo-jobs

### Task B1: bump the stardust dependency

- Modify: `cli/src/go.mod`, `cli/src/go.sum`

- [ ] Run `go get github.com/alxxpersonal/stardust@latest` in `cli/src`; resolve any breaking surface changes.
- [ ] `go build ./...` and `go test ./...` green.
- [ ] Commit `chore(deps): bump stardust to current`.

### Task B2: store uses the typed client

- Modify: `cli/src/internal/store/store.go`
- Test: `cli/src/internal/store/store_test.go`

- [ ] Write or extend a test that round-trips create, get, list, patch, delete through the store with no `map[string]any` in the path.
- [ ] Run it, confirm failure.
- [ ] Replace `sdk.Client` with `rpc.Client`; retype the record mapping to `rpc.Record`. Keep the `Store` public method shapes stable so callers are untouched.
- [ ] Run the test; loop to green.
- [ ] Gate (exo-jobs pre-commit hook runs vet, gofmt, golangci-lint), commit `refactor(store): consume the typed jsonrpc client`.

## Phase C - retire REST

### Task C1: remove the superseded REST handlers

- Modify: `internal/api/api.go`, `docs/openapi.yaml`

- [ ] Confirm no caller (exo-jobs, the Obsidian plugin, scripts) depends on the retired REST routes.
- [ ] Delete the REST `HandleFunc` registrations whose operations now live in the registry; keep `GET /healthz` and `POST /rpc`.
- [ ] Generate an OpenRPC document (`docs/openrpc.json`) and add an `rpc.discover` method to the registry; mark `docs/openapi.yaml` superseded with a header note pointing at the spec, do not delete it.
- [ ] `go build ./...`, `go test ./...`, `make lint` green; `grep` finds no retired routes.
- [ ] Commit `refactor(api): retire rest handlers superseded by the jsonrpc registry`.

## Phase D - .stardust docs in both repos

### Task D1: exo-jobs docs vault

- Create: `.stardust/` in the exo-jobs repo

- [ ] Run `stardust init --docs` in the exo-jobs repo root.
- [ ] Confirm `.stardust/config.toml`, the docs collections, the post-commit hook, and `docs/INDEX.md` exist.
- [ ] Commit `chore(docs): manage exo-jobs docs with stardust`.

### Task D2: stardust docs folders + registry

- Modify: stardust `docs/`

- [ ] Ensure `docs/specs`, `docs/plans`, `docs/adr`, `docs/research` exist (this spec, the ADRs, and this plan already populate them).
- [ ] Run `stardust registry` so `docs/INDEX.md` lists them.
- [ ] Commit `docs: regenerate index after the contract spec`.

## Phase E - contract pinning test suite

A suite that PINS the contract so any later change that breaks the wire shape, the method set, the error band, transport parity, or the schema fails a test. Built alongside Phase A; not optional polish.

### Task E1: golden wire-shape tests

- Create: `rpc/golden_test.go`, `rpc/testdata/<method>.json`

- [ ] For each method's `Params` and `Result`, marshal a representative value and compare against a checked-in golden JSON file; a field rename or shape change MUST fail.
- [ ] Run `go test ./rpc/ -run Golden`; loop to green.
- [ ] Gate, commit `test(rpc): pin method wire shapes with golden files`.

### Task E2: registry completeness

- Test: `internal/rpcserver/registry_test.go` (extend)

- [ ] Assert `NewRegistry` exposes EXACTLY the spec's slash method names, no more, no fewer; a removed or renamed method MUST fail.
- [ ] Run; loop to green.
- [ ] Gate, commit `test(rpcserver): pin the method set`.

### Task E3: error band

- Test: `internal/rpcserver/errors_test.go`

- [ ] Assert infrastructure errors fall in `-32000..-32099` and domain errors use positive codes (ADR 0006).
- [ ] Run; loop to green.
- [ ] Gate, commit `test(rpcserver): pin the error code band`.

### Task E4: transport parity

- Test: `internal/rpcserver/parity_test.go`

- [ ] Call `status` and a record round-trip over the jrpc2 stdio channel and the `jhttp` bridge; assert byte-identical results.
- [ ] Run; loop to green.
- [ ] Gate, commit `test(rpcserver): pin stdio and http transport parity`.

### Task E5: OpenRPC conformance

- Test: `internal/rpcserver/openrpc_test.go`

- [ ] Assert the generated OpenRPC document (`docs/openrpc.json`) lists exactly the registry's methods; doc drift MUST fail. (Runs once `rpc.discover` and the OpenRPC doc land in Phase C.)
- [ ] Run; loop to green.
- [ ] Gate, commit `test(rpcserver): pin openrpc document against the registry`.

## Verification

- Transport parity: `status` and a record round-trip return byte-identical typed results over stdio and `POST /rpc`.
- exo-jobs round-trips create/get/list/patch/delete through the typed client against a live daemon, no `map[string]any`.
- The MCP plugin's existing tool calls still succeed.
- After Phase C: no `mux.HandleFunc` for retired routes; `openapi.yaml` marked superseded.
- `go build ./...`, `go test ./...`, `make lint`, zero em or en dashes, in both repos.

## Self-review gate

- Every spec Work-breakdown item maps to a task above.
- Method, type, and field names are identical across tasks and match `rpc/contract.go`.
- No task removes REST before its consumers are migrated (ADR 0004 order holds).
- No placeholders; new code is shown or pointed at a real signature to confirm.
