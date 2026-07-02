---
title: Endpoint-free reranking via local-runtime discovery - implementation plan
status: Done
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-2324-endpoint-free-reranking.md
  - docs/adr/0044-reranking-discovers-local-runtimes-not-in-binary-onnx.md
---

Make reranking fire without a hand-configured endpoint by discovering a cross-encoder served by a local runtime the user already runs, with the fallback chain configured > discovered > none, then announce the active source and prove the pure-Go CGO-free release property holds. No inference runtime, no CGO, no model weights enter the binary. The reuse-Ollama future is pre-wired so it activates for free when upstream ships `/api/rerank`.

## Header

- **Goal:** with no `reranker_url` set, `service.Query` auto-discovers a rerank-capable local runtime, reranks through it, and announces the source; with a URL set, behavior is byte-identical to today. The required config surface shrinks to zero for the common case. A false positive to avoid: adopting a server that returns 200 but not a valid rerank body.
- **Architecture:** a new discovery step in front of the existing `internal/rerank` HTTP client resolves the reranker source once per service open as configured > discovered > none; discovery probes the already-configured Ollama host then the llama.cpp `llama-server` default with a two-document canary rerank, caches the first valid responder, and never blocks a query; `QueryResult` and `Status` gain a rerank-source signal (`configured` / `discovered` / `off` plus an off reason) extending the ADR 0016 loud-degradation rule to the rerank stage.
- **Tech stack:** Go 1.26, existing `internal/rerank` (`/v1/rerank` Jina / Cohere / llama.cpp client, graceful degradation), `internal/service` (`Query`, `Status`, `QueryResult`), `internal/config` (`RerankerURL` / `RerankerModel`), `internal/embed/ollama.go` (the cheap `Available` probe pattern to mirror). No new heavyweight dependency, no CGO, no model artifact, no new index table.
- **Global constraints:** ADR 0044 is normative (discover a runtime, never embed one; fallback configured > discovered > none; reuse Ollama the instant it can rerank; in-binary ONNX rejected). The `CGO_ENABLED=0` four-tarball-plus-brew release property must stay green and is regression-guarded by an import-boundary test. Conventional commits, imperative lowercase, no co-author or generated-by trailers, zero U+2014 / U+2013 anywhere, gate green before every commit.

## Context

Read first, in order: `docs/specs/2026-07-02-2324-endpoint-free-reranking.md` (Approach and Verification sections are normative), `docs/adr/0044-reranking-discovers-local-runtimes-not-in-binary-onnx.md` (the five decision rules and the struck in-binary alternative), `internal/rerank/rerank.go` (the `/v1/rerank` client and its degrade-to-input contract to keep intact), `internal/service/service.go` (`Query` rerank call site, `QueryResult`, `Status`, `RetrievalMode` / `RetrievalReason` as the announcement idiom to mirror), `internal/config/config.go` (`RerankerURL` / `RerankerModel` gate and defaults), and `internal/embed/ollama.go` (`Available`: probe cheaply, timeout-bound, cache, degrade, the template for discovery).

<details>
<summary><b>Task 1: reranker-source resolution and the discovery probe</b></summary>
<br>

Files:

- Modify: `internal/config/config.go` (document `reranker_url` as an optional override with a `none` sentinel that disables discovery; keep `reranker_model` an optional passthrough; no new required field)
- Create: `internal/rerank/discover.go` (the probe: an ordered candidate list, a two-document canary rerank, first-valid-responder wins, timeout-bound, cached; a `Resolve(cfg, ollamaURL)` that returns the source as configured > discovered > none)
- Create: `internal/rerank/discover_test.go` (fake runtimes via `httptest`: one answers the canary, one 404s, one times out, one returns 200 with a malformed body; assert only the valid responder is adopted, the sentinel disables probing, a configured URL skips discovery)
- Modify: `internal/rerank/rerank.go` only if needed to expose a canary-shaped call or a source enum; keep the existing `Rerank` degrade contract untouched

Steps:

- [x] Write `discover_test.go` first (test-driven): a configured URL is returned verbatim and no probe fires; `none` returns off and no probe fires; empty probes the candidate list and adopts the first server that returns a well-formed `results` array with a numeric score; a 404, a timeout, and a 200-with-garbage-body are all skipped; when none answer, the result is off with a reason.
- [x] Implement the probe: candidate order is the configured Ollama host first (rerank endpoint, yields nothing today), then `http://localhost:8080` (`/v1/rerank`); canary is documents `"a"` and `"b"` against query `"a"`; short per-candidate timeout; run at most once, cache the outcome; never return an error to the caller.
- [x] Implement `Resolve` returning the chosen URL (or none) plus a source tag (`configured` / `discovered` / `off`) and an off reason; keep the Ollama `/api/rerank` adapter a thin stub the probe never matches until upstream ships the endpoint, so the binary carries the seam without asserting an unmerged shape.
- [x] Confirm `internal/rerank` and the discovery package import only the standard-library net/http path (no CGO, no ONNX runtime, no model dependency).

</details>

<details>
<summary><b>Task 2: wire discovery into the service and announce the source</b></summary>
<br>

Files:

- Modify: `internal/service/service.go` (`Open` and `SetConfig` resolve the reranker source once via `rerank.Resolve` using `cfg` plus `cfg.OllamaURL`; `QueryResult` gains a rerank-source field and an off reason; `Status.Reranker` is joined by the source; `Query` announces the resolved source)
- Modify: `internal/service/query_test.go` and `internal/service/status_report_test.go` (assert the announced source for each branch: configured, discovered via a fake runtime, off with a reason)
- Modify: any surface that renders `QueryResult` or `Status` (CLI query output, status report) to show the source without changing the data contract shape beyond the additive field

Steps:

- [x] Add the additive rerank-source field to `QueryResult` (and a reason string when off) and to `Status`; do not remove or repurpose the existing `Reranked` boolean, which still means "order changed."
- [x] Resolve the source at `Open` and re-resolve at `SetConfig` (mirroring how `embed`/`rerank` clients are rebuilt today), caching the discovery outcome on the service so `Query` does not re-probe per call.
- [x] In `Query`, set the source on the result: `configured` when a URL was set, `discovered` when the probe found one, `off` with a reason otherwise; keep the FTS-only / hybrid-semantic `RetrievalMode` logic unchanged and orthogonal.
- [x] Update the CLI query and status renderers to surface the source line; keep JSON output additive so existing consumers do not break.

</details>

<details>
<summary><b>Task 3 (review): adversarial verification and the release-property gate</b></summary>
<br>

Files:

- Create: `internal/rerank/release_test.go` (import-boundary assertion: the rerank and discovery packages transitively pull no CGO dependency, so the default build stays static)
- Modify: test files as needed to close gaps found in review

Steps:

- [x] Prove the fallback chain end to end with fakes: configured wins and skips discovery; `none` disables both; discovered is adopted only for a valid canary responder; off is announced with a reason when nothing is found.
- [x] Prove safe degradation: a discovered runtime that later errors, times out, or returns malformed JSON returns the hybrid order unchanged and never fails `Query`.
- [x] Prove no regression for existing deployments: a vault with `reranker_url` set produces byte-identical rerank behavior to today; discovery is inert.
- [x] Run the full gate and confirm each passes unmasked: `go build ./...`, `go test ./...`, `make lint` exit 0, `gofmt -l .` empty, `CGO_ENABLED=0 go build ./...` passes, `stardust check` exit 0, and zero U+2014 / U+2013 in the diff. Regenerate `docs/INDEX.md` and flip this plan to Done and the spec to Implemented in the same commit as the code lands.

</details>

## Verification

The feature is done when, with no `reranker_url` configured and a local `llama-server --reranking` up, `stardust query` reranks through it and reports source `discovered`; with the server down it reports `off` with a reason and returns hybrid order; with `reranker_url` set it behaves exactly as today and reports `configured`; `CGO_ENABLED=0 go build ./...` and the import-boundary test both pass; and the config requires zero knobs for the common case. When Ollama ships `/api/rerank`, the same discovery over the already-configured Ollama host activates reranking with no further code change, which is the reuse-Ollama payoff ADR 0044 pre-wires.
