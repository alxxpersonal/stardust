---
title: Vectors on by default, loud degradation, incremental by content hash
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-doc-code-coherence-engine.md
---

# Vectors on by default, loud degradation, incremental by content hash

Semantic retrieval is on whenever Ollama is reachable, embeddings rebuild per changed chunk by content hash, a cross-encoder reranks the top-k, and any degrade to FTS-only is announced in the response.

## Context

There is no `vectors` config field; vectors are a runtime capability gated on `s.embed.Available(ctx)` (`internal/service/index.go:51`). When Ollama is unreachable the run degrades to FTS-only mid-run (`index.go:79`) and only an `IndexStats.Vectors bool` records it; the read path keeps consuming `Hybrid` as if semantic were live, so the tool silently serves worse results while still presenting as hybrid. The content-hash skip is note-level (`index.go:69`), so a one-character edit re-embeds every chunk of the note. The cross-encoder reranker exists (`internal/rerank/rerank.go`) but `RerankerURL` defaults empty (`config.go:34`) and nothing wires it into `Query`. Semantic plus rerank is the only reason to choose this over ripgrep, and it is dark.

## Decision

Three changes, all pure Go, none adding a required dependency:

1. Incremental per-chunk embedding. Persist a content hash per chunk derived from the exact embedded text (title plus heading plus body). On reindex, re-embed only chunks whose hash changed, replacing the note-level skip with chunk-level invalidation.
2. Loud degradation. `Query`, `Bundle`, and any future MCP surface carry a `retrieval_mode` field (`hybrid-semantic` or `fts-only`) and, when degraded, a one-line reason. The tool stops advertising hybrid while serving FTS.
3. Cross-encoder rerank. Wire `internal/rerank` into `Query` over the fused top-k when `RerankerURL` is set; it already degrades to identity when unreachable, announced under the same `retrieval_mode` discipline.

With no Ollama and no reranker the tool is an honest FTS engine; with both it is the hybrid-semantic-rerank engine the SPEC promises.

## Consequences

- Re-embedding cost drops to changed chunks only, making on-by-default affordable.
- Agents can trust the retrieval mode they were served and re-verify when degraded.
- A new per-chunk hash column is added to the index schema (a migration); `rebuild` always reconstructs from scratch, so the cache stays disposable.
- No cgo and no network vector store; brute-force cosine in Go is unchanged.

## Alternatives considered

- Force vectors on by erroring without Ollama. Rejected: local-first must degrade gracefully.
- Keep note-level hashing. Rejected: it makes embedding too expensive to leave on, which is why it is effectively off.
- Silent degrade with only the index stat. Rejected: the read consumer cannot see it, which is the current defect.

## References

- `internal/service/index.go`, `internal/index/search.go`, `internal/rerank/rerank.go`, `internal/config/config.go`, `internal/embed/ollama.go`.
- `SPEC.md` (hybrid-semantic-rerank promise).
