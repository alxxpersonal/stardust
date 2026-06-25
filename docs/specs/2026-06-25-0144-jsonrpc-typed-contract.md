---
title: JSON-RPC typed contract for stardust and exo-jobs
status: Draft
version: 1
date: 2026-06-25
related:
  - docs/openapi.yaml
  - docs/adr/0001-jsonrpc-canonical-transport.md
  - docs/adr/0002-typed-contract-package-in-stardust.md
  - docs/adr/0003-one-method-registry-multi-transport.md
  - docs/adr/0004-additive-then-retire-migration.md
  - docs/adr/0005-stardust-manages-docs-in-both-repos.md
---

# JSON-RPC typed contract for stardust and exo-jobs

Replace the stringly-typed REST + SDK seam between exo-jobs and stardust with one typed JSON-RPC 2.0 method registry served over both stdio (MCP) and HTTP, shared as a compiler-checked Go contract package, and set up `.stardust` to manage docs in both repos.

<details>
<summary><b>Problem</b></summary>
<br>

Stardust exposes one service core over two hand-maintained surfaces: a REST/JSON HTTP API (21 routes in `internal/api/api.go`, documented in `docs/openapi.yaml`) and an MCP server (`stardust serve --mcp`). exo-jobs consumes the REST surface through `sdk.Client` (`exo-jobs/cli/src/internal/store/store.go`), calling six methods (`CreateRecord`, `DeleteRecord`, `GetRecord`, `ListRecords`, `PatchRecord`, `Status`).

Three structural problems follow:

1. The record boundary is `map[string]any`. Field names are convention, not type. A renamed field is a silent runtime break across the repo seam.
2. The same operations are implemented twice, once as REST handlers and once as MCP handlers, so every new capability is wired in two places and can drift.
3. exo-jobs pins an 11-day-old stardust version (`v0.0.0-...85f8ffd`) in `cli/src/go.mod`, predating the docs-registry and agent-infra work, so its client is already behind the server.

MCP is JSON-RPC 2.0. The programmatic surface is REST. The stack runs two RPC styles for one core.
</details>

<details>
<summary><b>Context and background</b></summary>
<br>

- `stardust query "json-rpc http api transport"` and the README confirm the architecture: "stardust serve runs a localhost HTTP/JSON API over the same core the CLI uses" and "MCP server via stardust serve --mcp". One core, two transports.
- Operation inventory (from `internal/api/api.go` + `sdk/client.go`): read ops `query`, `bundle`, `graph`, `digest`, `check`, `status`, `note`, `collections`, `collection`, `records`, `record`, `mounts`, `cron`; write ops `createRecord`, `patchRecord`, `deleteRecord` (archive), `index`, `rebuild`, `archive`, `cronRun`.
- The transport decision was settled in conversation against REST-kept and gRPC. JSON-RPC won because the domain is verb-shaped and MCP already speaks it. See ADR 0001.
- exo-jobs already imports `github.com/alxxpersonal/stardust` as a private Go module, so the shared-package access pattern exists and needs no new tooling.
- Both repos follow the docs convention. stardust was scaffolded as a vault during exploration (`stardust init --docs`); exo-jobs has a `docs/` folder but no `.stardust`.
</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. One typed JSON-RPC 2.0 method registry is the canonical stardust contract, served over stdio (MCP) and HTTP (`POST /rpc`).
2. A shared Go package defines every method's params and result as typed structs with a contract version, imported by stardust (server) and exo-jobs (client).
3. exo-jobs talks to stardust only through the typed client; no `map[string]any` crosses the seam.
4. The REST handlers and the separate MCP handlers collapse into the single registry, then REST is retired.
5. `.stardust` manages docs in both repos, with the docs convention dogfooded on stardust itself.
</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No gRPC, no protobuf, no binary wire format (ADR 0001).
- No change to stardust's storage, index, FTS5, or vector internals. Only the transport and contract seam.
- No change to exo-jobs' game, TUI, ledger, or tax logic. Only its `store`/SDK layer.
- No auth or remote/multi-user surface. The server stays localhost, single-user.
- The exo-jobs `webhook` ingest stays plain HTTP (external feeds speak plain JSON, not the registry).
</details>

<details>
<summary><b>Approach</b></summary>
<br>

Four units.

**1. Typed contract package** (`stardust/rpc`). Per-method `Params` and `Result` structs for every operation, plus a `const ContractVersion`. Records become a typed `Record` struct, not `map[string]any`. This package is the single source of truth both binaries import.

**2. Method registry (jrpc2).** A `jrpc2` `handler.Map` keyed by slash noun/verb method name. Each handler decodes typed `Params`, calls the existing `internal/service` method, and returns a typed `Result`. One registration per operation, replacing the duplicated REST + MCP handler bodies. The library is `github.com/creachadair/jrpc2` (ADR 0006), not a hand-rolled dispatch and not the MCP go-sdk.

**3. Transport adapters (jrpc2 built-in + thin mcp adapter).** jrpc2 drives both transports over the one registry: stdio (NDJSON, with all logs routed to stderr so a stray write cannot corrupt the stream) and HTTP (`jhttp` at `POST /rpc`). A thin `mcp/` adapter at the edge maps the MCP framing (initialize handshake, capability negotiation, isError-in-result split) onto the registry for agent interop, without the MCP go-sdk's tool/resource cage. Plain `GET /healthz` stays a non-RPC liveness probe.

**4. Typed client.** `stardust/rpc` ships a `Client` wrapping jrpc2's client with typed methods. exo-jobs' `store` swaps `sdk.Client` for it.

Canonical method set (each maps a current route to a JSON-RPC method):

| JSON-RPC method | Replaces route | Params -> Result |
|---|---|---|
| `query` | `GET /query` | `QueryParams` -> `QueryResult` |
| `bundle` | `GET /bundle` | `BundleParams` -> `BundleResult` |
| `graph` | `GET /graph` | `GraphParams` -> `GraphResult` |
| `digest` | `GET /digest` | `DigestParams` -> `DigestResult` |
| `check` | `GET /check` | `CheckParams` -> `CheckResult` |
| `status` | `GET /status` | `{}` -> `StatusResult` |
| `note/get` | `GET /note` | `NoteParams` -> `Note` |
| `collection/list` | `GET /collections` | `{}` -> `[]Collection` |
| `collection/get` | `GET /collection` | `CollectionParams` -> `Collection` |
| `record/list` | `GET /records` | `ListRecordsParams` -> `RecordList` |
| `record/get` | `GET /record` | `RecordParams` -> `Record` |
| `record/create` | `POST /records` | `CreateRecordParams` -> `Record` |
| `record/patch` | `PATCH /record` | `PatchRecordParams` -> `Record` |
| `record/delete` | `DELETE /record` | `RecordParams` -> `DeleteResult` |
| `index/run` | `POST /index` | `IndexParams` -> `IndexStats` |
| `index/rebuild` | `POST /rebuild` | `{}` -> `IndexStats` |
| `index/archive` | `POST /archive` | `ArchiveParams` -> `ArchiveResult` |
| `mount/list` | `GET /mounts` | `{}` -> `[]Mount` |
| `cron/list` | `GET /cron` | `{}` -> `[]CronJob` |
| `cron/run` | `POST /cron/run` | `CronRunParams` -> `CronRunResult` |

Schema and discovery: an OpenRPC document is the contract source of truth, exposed at runtime via an `rpc.discover` method; it supersedes the REST `docs/openapi.yaml`. Errors follow the JSON-RPC codes, with the reserved `-32000..-32099` band for server and infrastructure failures and positive integers for domain errors (ADR 0006).

The implementer MUST confirm each `internal/service` signature in source before defining its `Params`/`Result`.

Docs setup: `stardust init --docs` is run in the exo-jobs repo to add `.stardust` + the docs collections; stardust's own `docs/specs`, `docs/plans`, `docs/adr`, `docs/research` folders are ensured to exist.
</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- **Keep REST, add types only.** Rejected. Leaves two surfaces (REST + MCP) drifting and does not unify the protocol; the typing win is available inside the JSON-RPC consolidation at the same cost.
- **gRPC + protobuf.** Rejected (ADR 0001). Binary wire loses curl-ability, needs an HTTP/2 + protoc toolchain, does not speak MCP, and the perf and streaming wins are irrelevant on localhost single-user.
- **Separate shared contract module.** Rejected (ADR 0002). A standalone module adds a second versioned repo to publish and pin. stardust already is the import target; the package lives there.
- **Connect (connectrpc).** Considered. Strong for HTTP + gRPC from one proto, but it still pulls protobuf and a codegen step for a two-Go-service localhost seam where hand-rolled typed JSON-RPC is simpler and keeps one protocol with MCP.
</details>

<details>
<summary><b>Risks</b></summary>
<br>

- MCP tool-shape compatibility. Coding agents consume specific MCP tool names and schemas; the registry MUST preserve the existing MCP tool surface or update the plugin in lockstep.
- Version skew during migration. The additive phase runs REST and JSON-RPC together; the contract `version` field and a handshake on `status` guard against a stale client silently mismatching.
- exo-jobs dependency bump. Moving from `85f8ffd` to current may surface unrelated breaking changes in the stardust module surface; the bump is its own task with a build gate.
- Scope creep into auth or remote. Explicitly out of scope; the registry stays localhost.
</details>

<details>
<summary><b>Open questions</b></summary>
<br>

- Should the typed `Client` also back the embedded CLI commands, or only exo-jobs and external callers? Default: migrate exo-jobs first, leave the CLI on its current path until proven.
- Does any MCP tool need a name that differs from its JSON-RPC method (for agent-facing clarity)? Resolve by reading the current MCP tool list before defining the registry.
- Keep `/metrics` as a second plain-HTTP endpoint, or defer until there is a metric to serve? Default: defer.
</details>

<details>
<summary><b>Verification</b></summary>
<br>

- Unit: every registry handler round-trips its typed `Params` and `Result` (table test over the method set).
- Transport parity: the same method called over stdio and over `POST /rpc` returns byte-identical typed results.
- Client: exo-jobs' `store` round-trips a record create, get, list, patch, delete through the typed client against a live daemon, with no `map[string]any` in the path.
- Compatibility: the MCP plugin's existing tool calls still succeed against the registry.
- Retirement: after REST removal, `grep` finds no `mux.HandleFunc` for the retired routes and `openapi.yaml` is marked superseded.
- Gates: `go build ./...`, `go test ./...`, `make lint`, zero em or en dashes, in both repos.
</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- Auth, TLS, remote access, multi-user.
- Streaming RPC (no current unbounded result set needs it).
- The exo-jobs webhook ingest path.
- Any storage, index, or embedding change.
</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. Create `stardust/rpc` contract package: typed `Params`/`Result` for the method set, `Record`, `ContractVersion`.
2. Build the method registry mapping names to handlers over `internal/service`.
3. Add the `POST /rpc` HTTP adapter; dispatch MCP stdio into the same registry.
4. Add the typed `rpc.Client`.
5. Migrate exo-jobs `store` to `rpc.Client`; bump the stardust dependency to current.
6. Retire the REST handlers; mark `openapi.yaml` superseded.
7. Set up `.stardust` in exo-jobs; ensure stardust docs folders exist.
</details>

<details>
<summary><b>References</b></summary>
<br>

- `docs/openapi.yaml` (the REST contract this supersedes)
- `internal/api/api.go`, `sdk/client.go`, `internal/service`
- exo-jobs `cli/src/internal/store/store.go`, `cli/src/go.mod`
- JSON-RPC 2.0 specification
- ADRs 0001 to 0006
</details>

## Amendments

### 2026-06-25: full-routes scope

The initial build was seam-first: the registry covered the six record-seam methods (record/create, record/get, record/list, record/patch, record/delete, status), which is exactly what exo-jobs consumes. This amendment expands the move to the FULL operation set so REST can be retired entirely. The remaining methods and their service signatures (confirmed in source):

| Method | Service call |
|---|---|
| query | Query(ctx, query, limit) |
| bundle | Bundle(ctx, task, budgetTokens) |
| graph | Graph(ctx) |
| digest | Digest(ctx, since, advance) |
| check | Check(ctx) |
| note/get | GetNote(ctx, path) |
| collection/list | ListCollections(ctx) |
| collection/get | GetCollection(ctx, name) |
| mount/list | Mounts() |
| index/run | Index(ctx, since) |
| index/rebuild | Rebuild(ctx) |
| archive | Archive(ctx, dest) |
| cron/list | CronList() |
| cron/run | CronRun(ctx, name, stardustBin, w io.Writer) |

Note: cron/run streams its output to an io.Writer. As a JSON-RPC method it MUST buffer the run output into a string result (a streaming notification variant is a later option); it is the one non-trivial mapping. Plan Phase F adds these methods; Phase C then retires all twenty-one REST routes plus the old sdk, and publishes the full OpenRPC document.
