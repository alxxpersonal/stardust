---
title: Cross-note contradictions surface as agent-judged candidates, never binary verdicts
status: Accepted
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-2234-cross-note-contradiction-candidates.md
  - docs/plans/2026-07-02-2234-cross-note-contradiction-candidates.md
  - docs/adr/0016-vectors-on-by-default-loud-degradation.md
  - docs/adr/0018-drift-detection-by-commit-distance.md
  - docs/adr/0019-ci-baseline-ratchet.md
  - internal/service/digest.go
  - internal/service/check.go
  - internal/index/search.go
  - internal/temporal/temporal.go
  - internal/embed/ollama.go
  - SPEC.md
---

# Cross-note contradictions surface as agent-judged candidates, never binary verdicts

Cross-note contradiction detection (SPEC section 11 future work, section 12.4 temporal behavior 3) splits along the charter line: the stardust binary deterministically prepares a short, high-precision list of contradiction candidate pairs, and an agent dispatch renders the semantic verdict. The binary never judges whether two statements conflict, never gains an LLM, and never asserts a contradiction as fact; every candidate it emits is a review prompt, weaker than a `check` warning, and lives outside the integrity check.

## Context

SPEC.md names this twice as unbuilt work: section 11 lists "LLM-based contradiction detection" among genuinely-future items, and section 12.4 lists "(3) contradiction detection across notes" as the third high-value temporal behavior beside the morning digest and commitment surfacing that already ship. Section 8 frames the felt problem: git history plus mount activity should let agents sense "this contradicts that."

The obstacle is a charter constraint, not a missing library. The binary has no LLM inference dependency and must never gain one: no API keys, no model runtime. Deciding whether "we will use Postgres" and "we migrated off Postgres" contradict, versus merely coexist across time, is a semantic judgment a deterministic binary cannot make without producing false accusations. A false contradiction flag on compatible notes is the failure mode that kills the feature: it trains the user to ignore the surface, exactly as a noisy linter trains a team to ignore warnings.

The tree already carries every primitive the deterministic half needs, so nothing heavyweight is added. `service.Digest` (`internal/service/digest.go:28-108`) keys off a stored commit cursor, diffs since it with `gitx.DiffNames`, groups changed notes by area, and scans bodies for commitments through `temporal.Commitments`. `index.Store.Hybrid` (`internal/index/search.go:39-97`) fuses FTS5 BM25 with brute-force cosine via RRF and degrades to FTS-only when no query vector is supplied. The embedder (`internal/embed/ollama.go`) is an optional Ollama probe that never hard-fails. Two precedents fix the surfacing discipline: ADR 0018 makes drift a `warn` phrased as a review prompt, never a hard failure, because a formatting-only commit can trip it; ADR 0016 requires any degrade to FTS-only be announced through `RetrievalMode` plus a reason, never silent. The librarian cron example (`docs/examples/cron-jobs/librarian/prompt.md`) is the agent-workflow pattern: read the CLI in a read-only sandbox, surface findings under notify / question / review, propose in `proposals/` rather than editing human notes. SPEC 12.3 records the sanctioned way to change your mind: append a new fact and stamp the old one `superseded_by` with `valid_from` / `valid_to`; a note that supersedes another that way is not a contradiction.

## Decision

Split contradiction detection into a deterministic candidate generator in the binary and a semantic judge in an agent dispatch, governed by five rules.

- **The binary prepares, never decides.** A new core method `service.Contradictions(ctx, opts)` generates ordered candidate pairs and returns a typed result plus rendered markdown; a new `stardust contradictions` command exposes it with `--since`, `--advance`, `--all`, `--limit`, and `--output`, matching the digest surface. No LLM, no API key, no model runtime enters the binary. The verdict is always an LLM call made by an agent that reads the candidates over the CLI.
- **Scope to the change feed, recall through existing retrieval.** The A-side is the set of notes changed since a contradiction-specific cursor `last_contradiction_sha`, computed with `gitx.DiffNames` exactly as digest computes its changed set, so the scan is bounded by recent activity, not vault size, and a new opposing statement becomes visible the moment it is written. For each assertion-bearing A-side chunk, recall same-subject B-side chunks through `index.Store.Hybrid` unchanged: hybrid-semantic when the embedder is up, FTS-only when it is not. The cursor is contradiction-specific, so a `digest --advance` never blinds the scan.
- **Precision is a conjunction of conservative gates, and the cap is a hard ceiling.** A recalled pair becomes a candidate only when all hold: different notes; a fused-score similarity floor (a shared subject); a shared-term Jaccard floor over non-stopword tokens (kills topically-near-but-different-subject pairs that embeddings alone would pair); polarity asymmetry, exactly one side carrying a reversal cue from a small curated lexicon extending `internal/temporal` (two statements that both assert or both negate are agreement, not contradiction); and benign exclusions, dropping pairs where one side supersedes the other through the sanctioned `superseded_by` / `valid_to` mechanism, or either side is a directory index, template, or the same collection record versioned over time. Survivors are ranked, deduped so `(A, B)` and `(B, A)` count once, and capped hard to a small top-N. A run that would emit hundreds is a run whose thresholds are wrong.
- **Every candidate is a review prompt, never a verdict, and never gates CI.** Output is phrased in the drift idiom (ADR 0018) with an explicit "candidate, not a verdict, likely benign; confirm before acting" hedge, and carries its evidence: the two note paths, the two opposing lines, the score, the shared terms, and which cue fired. Contradiction candidates are not a `check` kind and are not injected into `digest` output by default. Folding them into `check` would pollute the CI baseline ratchet (ADR 0019) and erode trust in the whole check surface; the candidate surface is advisory, weaker than a `warn`, and lives outside the integrity check. The primary signal degrades loudly: when the embedder is absent, the surface announces `fts-only` with a reason and returns fewer candidates rather than inventing a lower-quality signal (ADR 0016).
- **The LLM half is a cron-job example, propose-not-mutate.** A new `docs/examples/cron-jobs/contradiction-scan/` (prompt plus `kind = "agent"` config) in the librarian pattern runs `stardust contradictions`, pulls both notes for each candidate, judges with its LLM whether they genuinely conflict, and writes confirmed conflicts to `proposals/<date>-contradictions.md` under notify / question / review. It never edits human notes; files stay the source of truth; add-only. This is the whole LLM half: a prompt file plus a config, no binary inference.

## Consequences

- The charter line holds exactly: the binary stays deterministic and dependency-light (a lexicon plus a pair filter over the existing index, embedder, change feed, and hybrid retrieval), and the semantic judgment runs where an LLM already runs, in the agent dispatch.
- The false-accusation failure mode is defended in depth: the gate conjunction plus the hard cap keep the list short and on-subject, review-prompt framing never asserts a verdict, and the agent is a second semantic filter before anything reaches a proposal. A nominated non-contradiction costs one agent judgment, not a false verdict shown to the user.
- `stardust check` output and exit code are byte-identical before and after the feature; contradictions never appear there, so the CI ratchet stays clean and trustworthy.
- The scan is advisory and rerunnable: an embedder-down miss is recoverable on the next run, unlike a false accusation, so the design trades recall for precision by construction and biases every knob toward emitting nothing over emitting a wrong pair.
- The typed `Candidate` result is designed so an additive rpc method and MCP tool inherit it later (parity by construction, the one-core-many-transports pattern) when a non-CLI consumer appears; v1 ships CLI-only because the agent reads the CLI over Bash like the librarian does.
- One new lexicon and one new core method; no new heavyweight dependency, no new index table beyond the contradiction cursor in the existing `meta` store, no model runtime.

## Alternatives considered

- **Judge contradictions in the binary with a rule engine or an entailment classifier.** Rejected. Contradiction is a semantic call; a deterministic judge either underfits and misses real conflicts or overfits into false accusations, and a bundled classifier is the model runtime the charter forbids. The split keeps the binary deterministic and pushes the verdict to the agent.
- **Make contradiction candidates a `check` kind, like drift.** Rejected. `check` is integrity (broken links, malformed frontmatter, drift) and is CI-gateable through the baseline ratchet (ADR 0019). Contradiction candidates are a fuzzier, higher-false-positive surface that must never gate a commit; mixing them into `check` pollutes the ratchet and erodes trust in the whole check surface.
- **Inject candidates into the morning digest by default.** Rejected for v1. The digest is consumed as-is every morning and must stay clean; a noisy pair list dilutes it. The scan reuses the digest change feed but lives behind its own command; a one-line pointer in digest stays an additive future option.
- **Whole-vault pairwise similarity scan.** Rejected. O(notes squared) and a false-positive machine, because high similarity is agreement far more often than contradiction. Scoping the A-side to the change feed and recalling the B-side through existing retrieval bounds the work and raises precision.
- **Similarity alone as the signal.** Rejected. Two notes that agree on a topic are highly similar; similarity is necessary but nowhere near sufficient. Polarity asymmetry is the discriminating signal, and the shared-term floor keeps the pair on one subject.
- **A dedicated per-note contradiction vector or a trained model.** Rejected. New dependency and new index for a feature whose value is a short review list. Reuse of hybrid retrieval plus a lexicon is enough for v1.
- **A lexical-only whole-vault negation sweep when the embedder is down.** Deferred. FTS-seeded recall through `Hybrid` already covers the degraded case; a separate whole-vault lexical sweep would be noisier and is not worth the code. When the embedder is absent, the surface returns fewer candidates and says so.
- **Expose contradictions over rpc and MCP in v1.** Deferred. The agent reads the CLI over Bash like the librarian, so no non-CLI consumer exists yet; the additive rpc method and MCP tool follow the parity pattern when one appears.

## References

- SPEC.md section 11 (future work: "LLM-based contradiction detection"), section 12.4 (temporal behavior 3: "contradiction detection across notes"), section 8 ("this contradicts that"), section 12.3 (add-only, invalidate-not-delete, `superseded_by` / `valid_to`).
- ADR 0016 (vectors on by default, loud degradation), ADR 0018 (drift as a review prompt by commit-distance), ADR 0019 (CI baseline ratchet).
- `internal/service/digest.go:28-108` (cursor plus `DiffNames` change feed, `temporal.Commitments` body scan), `internal/temporal/temporal.go:12-43` (commitment regex, `TopArea`), `internal/index/search.go:39-199` (`Hybrid`, FTS-only degrade, `Nearest`, RRF), `internal/embed/ollama.go:37-100` (`Available`, batch `Embed`), `internal/service/check.go:14-208` (typed `Issue`, drift as `warn`), `internal/service/service.go:92-164` (`RetrievalMode`, `RetrievalReason`, `ftsOnlyReason`).
- `docs/examples/cron-jobs/librarian/prompt.md` and `config.toml` (propose-not-mutate, notify / question / review, read-only sandbox).
