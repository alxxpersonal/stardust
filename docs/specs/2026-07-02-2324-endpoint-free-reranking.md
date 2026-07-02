---
title: Endpoint-free reranking via local-runtime discovery
status: Implemented
version: 1
date: 2026-07-02
related:
  - docs/plans/2026-07-02-2324-endpoint-free-reranking.md
  - docs/adr/0044-reranking-discovers-local-runtimes-not-in-binary-onnx.md
  - docs/adr/0016-vectors-on-by-default-loud-degradation.md
  - internal/rerank/rerank.go
  - internal/service/service.go
  - internal/config/config.go
  - internal/embed/ollama.go
  - .goreleaser.yaml
  - SPEC.md
---

# Endpoint-free reranking via local-runtime discovery

Reranking is the second-biggest retrieval win in the SPEC and it is dead weight in the tree. It works only when the user sets `reranker_url` to a cross-encoder endpoint, and nobody does, so hybrid results are never re-scored. SPEC section 11 proposes fixing this with "a cross-encoder reranker served in-binary (pure-Go ONNX)." This spec surveys that proposal honestly, kills it, and locks the achievable replacement: the binary discovers a cross-encoder served by a local runtime the user already runs, with the fallback chain configured > discovered > none. No inference runtime, no CGO, no model weights enter the binary, and the required config knob goes away.

<details>
<summary><b>Problem</b></summary>
<br>

Two facts about the current tree set up the problem.

First, reranking already exists and already degrades safely. `internal/rerank/rerank.go` is a small HTTP client: it POSTs `{query, documents}` to `<reranker_url>/v1/rerank`, reads `{results:[{index, relevance_score}]}`, sorts by score, and on any error (no URL, unreachable, non-200, decode failure, empty results) returns the input order unchanged. `internal/service/service.go` `Query` calls `index.Store.Hybrid`, then calls `rerank.Rerank`, and records `QueryResult.Reranked` when the order changed. `Status` reports `Reranker: s.rerank.Enabled()`.

Second, it is gated behind a knob nobody sets. `internal/config/config.go` defines `RerankerURL` (`reranker_url`, "optional cross-encoder endpoint; empty = disabled") and `RerankerModel`, both defaulting to empty. `rerank.Client.Enabled()` returns `url != ""`. Because the default vault has no reranker endpoint and standing one up plus naming its URL is friction, the second-biggest retrieval win (SPEC section 10.2, a cross-encoder over the top ~50-100 hits down to the returned k) never fires in practice.

The felt pain is not the model, it is the endpoint configuration. So the design question is how to make reranking fire without the user hand-configuring a separate server, without breaking the pure-Go single-static-binary release property, and without adding a heavyweight dependency or a model artifact to the binary.

</details>

<details>
<summary><b>Context and background</b></summary>
<br>

The constraints that shape every option are already in the tree.

- **The release property is a hard asset.** `.goreleaser.yaml` builds with `CGO_ENABLED=0` for darwin and linux on amd64 and arm64: four static tarballs plus a Homebrew formula. SPEC section 9 chose pure-Go `modernc.org/sqlite` and brute-force cosine in Go precisely to keep this single-binary, trivial-cross-compile property. Any reranking mechanism that requires CGO or a runtime-loaded shared library for the default build forfeits it.
- **The embedder is already an optional local runtime over HTTP.** `internal/embed/ollama.go` talks to Ollama with a cheap `Available` probe (GET `/api/tags`, 3s timeout, checks the model is pulled) and retrieval degrades to FTS-only when it is absent, never hard-failing. Discovery reuses this exact "probe cheaply, cache, degrade loudly" philosophy.
- **Loud degradation is already law.** ADR 0016 requires any degraded stage to announce itself; `service.go` surfaces `RetrievalMode` (`hybrid-semantic` vs `fts-only`) and a `RetrievalReason`. The rerank stage must announce its source the same way, so "no runtime found" is never confused with "reranker ran."
- **The rerank contract is already the industry one.** `internal/rerank` speaks `/v1/rerank` with `{query, documents}` in and `{results:[{index, relevance_score}]}` out, the Jina / Cohere / llama.cpp shape. Whatever discovery finds must speak a known contract, and llama.cpp already speaks this one.

The library survey below was run against the upstream repositories in July 2026, not from memory, because the whole verdict turns on what can actually run a cross-encoder in or beside a pure-Go binary today.

</details>

<details>
<summary><b>Library survey (verified July 2026)</b></summary>
<br>

**Can a cross-encoder run inside the pure-Go binary?** A cross-encoder reranker is a BERT-class sequence-classification model: it jointly encodes the query and each document and emits a single relevance logit. Running one in-process needs an ONNX (or GGUF) runtime with full transformer operator coverage plus a tokenizer.

- `yalue/onnxruntime_go` wraps `microsoft/onnxruntime`. It requires CGO and a runtime `libonnxruntime` shared library per platform (on Linux the versioned `.so`, not the bare symlink). That breaks the letter of the `CGO_ENABLED=0` property: the default static binary would not build, and the tarball would need a co-shipped native library.
- `knights-analytics/hugot` now exposes a `crossEncoder` pipeline (reranking is in scope, unlike a year ago). Its performant backends are ORT (CGO plus `libonnxruntime.so`) and XLA (CGO plus XLA libraries plus a Rust `libtokenizers.a`). It also has a CGO-free pure-Go backend powered by GoMLX, but the maintainers document it as "for simpler workloads, environments that disallow cgo, and for smaller models such as all-MiniLM-L6-v2," best with batches of roughly 32, and explicitly recommend moving to a C backend for performance. bge-reranker-v2-m3 (~560M) and Qwen3-Reranker-0.6B, the models SPEC section 10.2 names, sit above that lighter tier. The pure-Go path additionally pulls the GoMLX framework as a dependency and still requires the ONNX model weights to be distributed (bundled = a much heavier binary plus a model license, or downloaded on first use = a network dependency plus a cache directory plus a new implicit knob). That is new dependency weight and new artifact plumbing, the opposite of the charter's dependency-light binary.
- `oramasearch/onnx-go` on Gorgonia does not implement the full ONNX operator set; the project's own docs note "most models from the model zoo will not work." A BERT-class cross-encoder will not run. Effectively stale for this purpose.

Conclusion: a lean, dependency-light, model-distribution-free, CGO-free in-binary cross-encoder does not exist today. The SPEC section 11 "pure-Go ONNX in-binary" line is a myth for this binary. See ADR 0044.

**Can the already-configured Ollama rerank?** This is the cheapest possible win if it works, because Ollama is already the optional embedder dependency, so reusing it adds zero config for existing semantic users.

- Ollama exposes embeddings (`/api/embed`, already used) but has no rerank endpoint. PR ollama/ollama#7219, which proposed `/api/rerank`, was closed unmerged on 2025-09-16 as idle. The follow-up PR #11156 and issue #16076 (May 2026) confirm there is still no `/api/rerank`; the tracking request #10989 has been open since June 2025 at roughly 70 upvotes with no maintainer engagement.
- The only workarounds are yes/no token log-likelihoods via `/api/generate` (awkward and slow) or scoring query-document embedding cosine. The cosine workaround uses the wrong layer of a cross-encoder and adds nothing over the bge-m3 vectors hybrid retrieval already fuses via RRF.

Conclusion: reusing Ollama for a true cross-encoder rerank does not work now. But it is the obvious future once the endpoint lands, and the design must be pre-wired to capture it for free.

**What actually reranks over the contract the tree already speaks?** llama.cpp `llama-server` serves `/v1/rerank` when launched with `--reranking` (mutually exclusive with `--embeddings`) and `pooling=rank`. bge-reranker-v2-m3 (Q4_K_M GGUF) is the reference well-behaved model. Results come back in input order and are sorted by `relevance_score` client-side, which `internal/rerank` already does. The only gap is that the user must hand-configure the URL.

</details>

<details>
<summary><b>Goals and non-goals</b></summary>
<br>

**Goals**

1. Reranking fires without the user hand-configuring a separate reranker endpoint whenever a rerank-capable local runtime is up.
2. The pure-Go, CGO-free, single-static-binary release property is preserved by construction and guarded against regression.
3. The reuse-Ollama future (Ollama gaining `/api/rerank`) activates with zero user action and zero new dependency the day upstream ships it.
4. The active rerank source is announced, never silent, so a degraded state is legible.
5. The config surface ends strictly simpler: no new required knob, and `reranker_url` demoted from required to optional override.

**Non-goals**

1. No inference runtime, ONNX or GGUF, embedded in the binary. No CGO backend. No bundled or downloaded model weights.
2. No new rpc method or MCP tool in this iteration; reranking is an internal retrieval stage, already reachable through every surface that calls `service.Query`.
3. No attempt to make reranking work when no runtime is present. Absent a runtime, the surface stays off and says so, exactly as today minus the dead knob.
4. No promise of a specific latency number; expectations are stated as hardware-dependent ranges.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

Ship endpoint-free reranking as local-runtime auto-discovery in front of the existing HTTP `rerank.Client`. The mechanism is a probe, a fallback chain, a contract adapter set, and an announced source.

**1. Discovery probe.** Add a discovery step invoked when `reranker_url` is unset. It walks a short, fixed, ordered list of candidate endpoints, sends each a two-document canary rerank (documents `"a"` and `"b"` against query `"a"`), and caches the first that returns a well-formed `results` array carrying a numeric relevance score. The candidate list, in priority order:

1. The already-configured Ollama host, probed for a rerank endpoint. Today this yields nothing (one cached 404); it is the forward-compatible seam that captures the Ollama-rerank future for free.
2. The llama.cpp `llama-server` default (`http://localhost:8080`), which speaks `/v1/rerank` today.

The probe runs at most once per service open, only when no URL is configured, is cached for the service lifetime, and never blocks or fails a query. This mirrors `embed.Available`: cheap, timeout-bounded, safe to fail.

**2. Fallback chain: configured > discovered > none.**

- If `reranker_url` is a real URL, use it verbatim (override and escape hatch). This preserves every current deployment.
- If `reranker_url` is the sentinel `none`, hard-disable both the reranker and discovery, for users who do not want localhost probing.
- If `reranker_url` is empty (the default), run discovery. Use the discovered runtime if one answered; otherwise reranking is off.
- With no source, `index.Store.Hybrid` order is returned unchanged.

**3. Contract adapters.** `/v1/rerank` (llama.cpp, Jina, Cohere) is the concrete contract shipped, and `internal/rerank` already speaks it. The Ollama probe is wired to whatever `/api/rerank` shape Ollama finalizes; until it merges, the Ollama adapter is a thin, test-gated stub that the probe simply never matches, so the binary carries the seam without asserting an unmerged API's exact fields.

**4. Announced source (loud degradation, ADR 0016).** `QueryResult` gains a rerank-source signal alongside `Reranked`: one of `configured`, `discovered`, or `off`, plus a reason when off (for example, "no reranker configured and no local runtime discovered"). `Status` reports the same. A consumer can then distinguish "a runtime reranked" from "nothing was found," which the current single `Reranked` boolean cannot express.

**5. Config simplification.** `reranker_url` is documented as an optional override with `none` to disable discovery, no longer a required enable knob. `reranker_model` stays an optional passthrough hint for a runtime serving multiple models. No new required field is introduced; the required surface shrinks to zero for the common case.

**Release-property guard.** Discovery and reranking stay pure net/http plus JSON. A test asserts the `internal/rerank` and discovery packages transitively import no CGO path, so a later edit cannot pull a native dependency into the default build. The existing `CGO_ENABLED=0 go build ./...` gate stays green, and goreleaser still emits four static tarballs plus a working brew formula.

</details>

<details>
<summary><b>Quality and latency expectations</b></summary>
<br>

Quality: a cross-encoder rerank over the top ~50-100 hybrid hits down to the returned k is SPEC section 10.2's second-biggest retrieval win, fully local, ahead of hybrid alone because a joint query-document encoder resolves relevance that bi-encoder cosine plus BM25 blur. bge-reranker-v2-m3 is the reference model; discovery does not change the quality ceiling, it removes the configuration barrier in front of it.

Latency: hardware-dependent and stated as a range, not a promise. A local bge-reranker-v2-m3 (Q4_K_M GGUF) reranking ~50-100 short candidate texts on CPU is typically in the low hundreds of milliseconds to a couple of seconds, with a slower first call while the model loads. The existing 30s client timeout is generous, and because reranking is best-effort and degrades to input-unchanged on any error or timeout, a slow or cold runtime never blocks a query. The discovery probe itself is timeout-bounded (a short canary) and runs once per service open, so its cost is a single small round-trip, not a per-query tax.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- **Fallback chain.** A configured `reranker_url` is used verbatim and discovery does not run. `none` disables both reranker and discovery. Empty runs discovery: a fake runtime that answers the canary is adopted; a fake that 404s or times out is skipped; when none answer, reranking is off and the announced source is `off` with a reason.
- **Announced source.** `QueryResult` and `Status` report `configured` / `discovered` / `off` correctly for each branch, and the off reason is present. The existing `Reranked` flag still reflects whether order actually changed.
- **Degradation is safe.** A discovered runtime that later errors, times out, or returns malformed JSON returns the hybrid order unchanged and never fails the query. The canary is a real rerank shape, so a server that returns 200 but the wrong body is not adopted.
- **Release property.** `CGO_ENABLED=0 go build ./...` passes. An import-boundary test asserts `internal/rerank` and the discovery package pull no CGO transitive dependency. `gofmt -l` is empty, `make lint` exits 0, `stardust check` exits 0, and there are zero U+2014 / U+2013 runes in the new code and docs.
- **No behavior change for existing deployments.** A vault with `reranker_url` already set produces byte-identical rerank behavior to today; discovery is inert when a URL is configured.

</details>
