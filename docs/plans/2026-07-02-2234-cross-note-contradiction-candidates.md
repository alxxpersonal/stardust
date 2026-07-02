---
title: Cross-note contradiction candidates - implementation plan
status: Draft
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-2234-cross-note-contradiction-candidates.md
  - docs/adr/0043-contradiction-candidates-not-verdicts.md
---

Build a deterministic contradiction-candidate generator in the binary that reuses the git change feed, hybrid retrieval, and the optional embedder to emit a short, high-precision list of review prompts, plus the librarian-pattern cron example that reads those candidates and renders the LLM verdict, then adversarially prove agreement, off-subject, and superseded pairs never fire, the cap holds, degradation is loud, and the `check` surface is untouched.

## Header

- **Goal:** `stardust contradictions` deterministically surfaces a small number of high-quality contradiction candidate pairs as review prompts (never verdicts), and a `contradiction-scan` cron agent judges them and files proposals. Precision beats recall at every knob; a false pair on compatible notes is the failure mode.
- **Architecture:** one core method `service.Contradictions(ctx, opts)` returns a typed `Candidate` slice plus rendered markdown; a polarity lexicon extends `internal/temporal`; the A-side rides the git change feed behind a contradiction-specific cursor `last_contradiction_sha`; the B-side recalls through `index.Store.Hybrid` unchanged; a pure pair-gate filter enforces precision; the CLI command mirrors the digest surface; the LLM half is a prompt plus config in the librarian pattern. Not a `check` kind, not injected into digest.
- **Tech stack:** Go 1.26, existing `internal/gitx` (`DiffNames`, `HeadSHA`), `internal/index` (`Hybrid`, `meta` cursor store), `internal/embed` (optional Ollama), `internal/temporal` (lexicon home). No new heavyweight dependency, no LLM in the binary, no new index table beyond the cursor meta key.
- **Global constraints:** ADR 0043 is normative (candidates not verdicts, binary prepares and agent judges, review-prompt framing, not a check kind, loud degradation). No LLM inference dependency, no API key, no model runtime in the binary. Conventional commits, imperative lowercase, no co-author or generated-by trailers, zero U+2014 / U+2013 anywhere, gate green before every commit.

## Context

Read first, in order: `docs/specs/2026-07-02-2234-cross-note-contradiction-candidates.md` (Approach and Verification sections are normative), `docs/adr/0043-contradiction-candidates-not-verdicts.md` (the five decision rules and the gate conjunction), `internal/service/digest.go` (`Digest` is the cursor-plus-`DiffNames` change-feed template to copy for the A-side), `internal/temporal/temporal.go` (`Commitments` regex and `TopArea` are the lexicon and area home to extend), `internal/index/search.go` (`Hybrid` for B-side recall, its FTS-only degrade, RRF), `internal/service/service.go` (`RetrievalMode` / `RetrievalReason` / `ftsOnlyReason` and `embedQuery` are the degradation-visibility and embed-reuse seams to mirror), `internal/embed/ollama.go` (`Available`, batch `Embed`), `internal/service/check.go` (the typed `Issue` review-prompt idiom to echo without becoming a check kind), and `docs/examples/cron-jobs/librarian/prompt.md` plus `config.toml` (the propose-not-mutate agent pattern to clone).

## Task 1: deterministic candidate generator, CLI command, and cron example

Files:

- Modify: `internal/temporal/temporal.go` (add the polarity lexicon: reversal cues and assertion markers, plus an exported `AssertionKind` / anchor detector; keep the existing commitment regex intact)
- Create: `internal/service/contradictions.go` (`Contradiction` candidate type, `ContradictionsResult`, the `Contradictions` core method, the pure pair-gate filter, the rank/cap/dedupe, and the markdown renderer)
- Create: `internal/service/contradictions_test.go` (unit tests for the pair gates and the rank/cap/dedupe, with a fake embedder for the hybrid path and an FTS-only path)
- Create: `internal/cli/contradictions.go` (the `stardust contradictions` command with `--since`, `--advance`, `--all`, `--limit`, `--output auto|md|json|plain`, mirroring the digest command)
- Create: `docs/examples/cron-jobs/contradiction-scan/prompt.md` (librarian-pattern agent: run the CLI, judge each candidate, file proposals under notify / question / review, never edit human notes)
- Create: `docs/examples/cron-jobs/contradiction-scan/config.toml` (`kind = "agent"`, read-only sandbox, a cadence matching the librarian)
- Optional: `internal/temporal/temporal_test.go` (lexicon anchor-detection cases) if not already covered

Steps:

- [ ] Add the polarity lexicon to `internal/temporal`: a reversal-cue set (`not`, `no longer`, `never`, `isn't`/`aren't`/`won't`/`can't`/`don't`, `deprecated`, `reverted`, `rolled back`, `cancelled`, `abandoned`, `dropped`, `removed`, `obsolete`, `superseded`, `replaced by`, `instead of`, `decided against`, `changed our mind`) and an assertion-marker set (`decided`, `we will`, `chose`, `locked`, `canonical`, `must`, `always`, `default`). Export an anchor detector that classifies a line as assertion-bearing, reversal-bearing, or neither. Unit-test it first.
- [ ] Add the contradiction-specific cursor: read/write `last_contradiction_sha` through the index `meta` store, wired exactly like `last_digest_sha` in `Digest`, and never share the digest cursor.
- [ ] Write `contradictions_test.go` before the core (test-driven): a real conflict fires exactly one candidate; two agreeing notes fire none; two topically-near-but-different-subject notes (one with a reversal cue) fall below the shared-term floor and fire none; a `superseded_by` / `valid_to` pair fires none; a constructed flood returns at most top-N, ranked, deduped so `(A, B)` and `(B, A)` appear once; embedder-up reports `hybrid-semantic`, embedder-down reports `fts-only` with a reason.
- [ ] Implement `Contradictions(ctx, opts)`: resolve the A-side from the cursor (or `--since` / `--all`) with `gitx.DiffNames`, filter A-side chunks to anchors via the lexicon, recall same-subject B-side chunks through `index.Store.Hybrid` seeded with the anchor's salient terms (reusing the `embedQuery` seam so the vector is computed once), apply the pure pair-gate filter (different notes, similarity floor, shared-term Jaccard floor, polarity XOR, benign-superseded/index/template exclusions), then rank, dedupe symmetric pairs, and cap to top-N with a per-anchor cap.
- [ ] Populate the typed `Contradiction{NoteA, LineA, NoteB, LineB, Score, SharedTerms, Cue, RetrievalMode, RetrievalReason}` and render each as an explicit review prompt with the "candidate, not a verdict, likely benign; confirm before acting" hedge; inherit `RetrievalMode` / `RetrievalReason` from the recall so degradation is loud (ADR 0016).
- [ ] Wire `stardust contradictions` in `internal/cli`: `--since`, `--advance` (move the cursor to HEAD after a run), `--all` (full-vault A-side, off by default, documented cost), `--limit`, `--output`. Do not register it as a `check` kind and do not inject it into `digest` output.
- [ ] Add `docs/examples/cron-jobs/contradiction-scan/` (prompt plus config) in the librarian pattern: the agent runs the CLI, pulls both notes per candidate, judges with its LLM, writes confirmed conflicts to `proposals/<date>-contradictions.md` under notify / question / review, add-only, never editing human notes.
- [ ] Gate: `go build ./...`, `go test ./...`, `make lint` (exit 0, unmasked), `gofmt -l .` empty, dash-scan (zero U+2014 / U+2013 in every touched file), `stardust check` exit 0.
- [ ] Commit `feat(digest): surface cross-note contradiction candidates as review prompts`.

## Task 2: adversarial precision review

Steps:

- [ ] Real-conflict proof: two notes, one asserting a decision and a later one reversing it over the same subject, produce exactly one candidate naming both lines, the shared terms, and the cue.
- [ ] Agreement proof: two notes that agree on the same subject (both assert, high similarity) produce no candidate, proving similarity alone is inert without polarity asymmetry.
- [ ] Off-subject proof: two topically near notes about different subjects, one carrying a reversal cue, fall below the shared-term floor and produce no candidate.
- [ ] Superseded proof: a note that supersedes another through `superseded_by` or `valid_to` produces no candidate, honoring the sanctioned change-your-mind path (SPEC 12.3).
- [ ] Cap proof: a constructed vault that would emit hundreds of pairs returns at most top-N, ranked, deduped so `(A, B)` and `(B, A)` appear once.
- [ ] Degradation proof: with the embedder up a scan reports `retrieval_mode: hybrid-semantic`; with it down the same scan reports `fts-only` with a reason, never silently.
- [ ] Cursor-independence proof: a `digest --advance` between two contradiction scans does not change what the second scan sees; only `contradictions --advance` moves its cursor.
- [ ] Not-a-check proof: `stardust check` output is byte-identical before and after the feature and its exit code is unaffected; contradictions never appear there.
- [ ] Framing proof: every rendered candidate is phrased as a review prompt with the explicit "candidate, not a verdict, likely benign" hedge; no output asserts a contradiction as fact.
- [ ] Agent-workflow proof: the `contradiction-scan` prompt reads candidates through the CLI, writes proposals under notify / question / review, and never edits a human note.
- [ ] Verify `git log` shows clean conventional commits with no trailers; dash-scan every touched file; report defects, do not fix silently.

## Verification

The spec's Verification cases green; a real reversal fires exactly one candidate while agreement, off-subject, and superseded pairs fire none; the cap is a hard ceiling with symmetric dedupe; degradation to FTS-only is announced with a reason; the contradiction cursor is independent of the digest cursor; `stardust check` is byte-identical and its exit code untouched; every candidate reads as a review prompt, never a verdict; the cron example proposes and never mutates; gate clean (`go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` empty, zero U+2014 / U+2013, `stardust check` exit 0).

## Self-review gate

- Every ADR 0043 gate (different notes, similarity floor, shared-term floor, polarity XOR, benign exclusions, hard cap) maps to a `contradictions_test.go` case, and the false-positive classes (agreement, off-subject, superseded) each have an explicit no-fire test.
- The binary gained no LLM, no API key, and no model runtime; the only semantic judgment lives in the cron agent, which reads the CLI and proposes, never mutates.
- Contradiction candidates are not a `check` kind and are not in `digest` by default; `check` output and exit code are unchanged, keeping the CI ratchet (ADR 0019) clean.
- Degradation is loud: the surface inherits `RetrievalMode` / `RetrievalReason` and announces FTS-only, never silently serving a weaker result (ADR 0016).
