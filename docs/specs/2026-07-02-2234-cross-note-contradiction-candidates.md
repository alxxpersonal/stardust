---
title: Cross-note contradiction candidates
status: Implemented
version: 1
date: 2026-07-02
related:
  - docs/plans/2026-07-02-2234-cross-note-contradiction-candidates.md
  - docs/adr/0043-contradiction-candidates-not-verdicts.md
  - docs/adr/0016-vectors-on-by-default-loud-degradation.md
  - docs/adr/0018-drift-detection-by-commit-distance.md
  - docs/examples/cron-jobs/librarian/prompt.md
  - internal/service/digest.go
  - internal/service/check.go
  - internal/index/search.go
  - internal/embed/ollama.go
  - internal/temporal/temporal.go
  - internal/gitx/gitx.go
  - SPEC.md
---

# Cross-note contradiction candidates

Two notes in the same vault can assert incompatible facts and sit unnoticed forever: a decision doc says "we will use Postgres", a later note says "no longer using Postgres", and nothing connects them. Stardust should surface these, but the binary cannot judge whether two statements truly conflict, because that is a semantic call and the binary carries no LLM. The design splits the problem: the binary deterministically prepares a small, high-precision set of candidate pairs, and an agent dispatch judges them. This spec locks the deterministic half and the agent workflow that consumes it.

<details>
<summary><b>Problem</b></summary>
<br>

SPEC.md names this as unbuilt future work in two places. Section 11 lists "LLM-based contradiction detection" among the genuinely-not-yet-built items. Section 12.4 lists it as the third high-value temporal behavior: "(3) contradiction detection across notes", alongside the morning digest and commitment surfacing that are already built. Section 8's temporal pitch frames the felt problem directly: git history plus mount activity should give agents a sense of "this contradicts that".

The obstacle is a charter constraint, not a missing library. The stardust binary has no LLM inference dependency and must never gain one: no API keys, no model runtime. Judging whether "we will use Postgres" and "we migrated off Postgres" contradict, versus merely coexist across time, is a semantic judgment. A deterministic binary cannot make it without producing false accusations, and a false contradiction flag on compatible notes is the failure mode that kills the feature: it trains the user to ignore the surface, exactly as a noisy linter trains a team to ignore warnings.

So the binary's job is to prepare and surface, not to decide. It must generate candidate pairs cheaply and conservatively from signals it can compute (embedding similarity from the optional embedder, lexical polarity markers, the git change feed), hand a short list to an agent, and let the agent's LLM render the verdict. The design question is which deterministic signals yield a small number of high-quality candidates, how they surface as review prompts rather than verdicts, and how the agent half reads them through the surface the binary already exposes.

</details>

<details>
<summary><b>Context and background</b></summary>
<br>

Stardust is local-first, git-backed, markdown-truth. The SQLite index, link graph, and embeddings are disposable caches regenerated from files. The constraints that shape every option below are already in the tree:

- No LLM in the binary. Deterministic work only: lexical and structural heuristics plus embedding similarity through the existing optional embedder (`internal/embed/ollama.go`). The embedder is Ollama over HTTP with a cheap `Available` probe; retrieval degrades to FTS-only when it is absent, and never hard-fails on it.
- Git is already the change feed. `internal/gitx/gitx.go` exposes `DiffNames`, `HeadSHA`, `LastCommit`, `CommitCountSince`. `service.Digest` (`internal/service/digest.go`) already keys off a stored cursor (`last_digest_sha`), diffs since it with `DiffNames`, groups changed notes by area, and scans their bodies for commitment lines through `temporal.Commitments`. Contradiction candidates ride the same rail: recently changed notes are the natural place a new statement that opposes an old one appears.
- Hybrid retrieval already exists. `index.Store.Hybrid` (`internal/index/search.go`) fuses FTS5 BM25 with brute-force cosine over stored vectors via RRF, collapses to the best chunk per note, and degrades to FTS-only when no query vector is supplied. `index.Store.Nearest` returns cosine-nearest notes and is the dedup-before-write primitive. Both are reusable for "find the notes that talk about the same subject as this statement".
- The check pipeline is the model for surfacing findings as review prompts. `service.Check` (`internal/service/check.go`) emits typed `Issue{Severity, Kind, Path, Detail}` values. Drift (ADR 0018) is a `warn` of kind `drift`, phrased as a review prompt and never a hard failure, precisely because a formatting-only commit can trip it. Contradiction candidates inherit that discipline: they are review prompts by construction, weaker than drift, because the false-positive surface is larger.
- The librarian cron example is the agent-workflow pattern. `docs/examples/cron-jobs/librarian/prompt.md` runs an agent against the stardust CLI in a read-only sandbox, surfaces findings in a notify / question / review taxonomy, writes proposals rather than editing human notes, and prefers add-only edits. `config.toml` wires it as a scheduled `kind = "agent"` job. The contradiction agent is a sibling of the librarian: same propose-not-mutate contract, narrower job.
- The memory model already sanctions superseding. SPEC 12.3 records the add-only, invalidate-not-delete conflict policy: append a new fact and stamp the old one `superseded_by` with frontmatter `valid_from` / `valid_to`, letting retrieval surface the current version. A note that supersedes another through that mechanism is not a contradiction; it is the sanctioned way to change your mind, and must be excluded from candidates.

Go conventions apply (`.claude/rules/go.md`): `%w` wrapping, doc comments on exports, `// --- Section ---` separators, no em or en dashes anywhere.

No external research was required; every claim here is grounded in the current source tree, file cited.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. The binary generates a short, ranked list of contradiction candidate pairs deterministically, with no LLM and no new heavyweight dependency, reusing the index, the embedder, the git change feed, and hybrid retrieval.
2. Precision beats recall at every knob. A handful of high-quality candidates is the target; a wall of plausible-but-wrong pairs is a failure, because it trains the user to ignore the surface.
3. Every candidate is framed as a review prompt, never a verdict, mirroring how drift findings are review prompts by design (ADR 0018).
4. Candidates carry their evidence: the two note paths, the two opposing lines, the similarity score, the shared terms, and which polarity cue fired, so a human or an agent can judge without re-deriving the signal.
5. The agent half is a cron-job example (prompt plus config) in the librarian pattern: it reads candidates through the CLI the binary already exposes, judges each with its LLM, and files proposals under the notify / question / review taxonomy, propose-not-mutate, add-only.
6. The primary signal degrades loudly. When the embedder is absent, the surface announces the degraded mode rather than silently serving a weaker or empty result, mirroring the retrieval-mode discipline (ADR 0016).
7. The candidate surface never gates CI and never blocks a commit. It is advisory, weaker than the `warn`-level check surface, and lives outside the integrity check.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- Judging contradictions in the binary. The binary prepares candidates; the verdict is always an LLM call in an agent dispatch. No API key, no model runtime, no inference dependency enters the binary. This is a hard charter line.
- Auto-editing notes to resolve a contradiction. The agent proposes; the human decides. Add-only, invalidate-not-delete (SPEC 12.3).
- A whole-vault semantic diff or an entailment model. The candidate set is scoped and capped; it is not an exhaustive pairwise scan.
- Temporal reasoning about validity windows in the binary. If a note supersedes another through the sanctioned frontmatter mechanism, the pair is excluded, not reasoned about.
- Making this a `check` kind or a CI gate. Contradiction candidates are advisory review prompts, not integrity errors; folding them into `check` would pollute the CI ratchet (ADR 0019) and train users to ignore check output.
- rpc and MCP exposure in v1. The agent reads candidates through the CLI over Bash, exactly as the librarian reads `stardust digest`. An additive rpc method and MCP tool follow the established parity pattern later, when a non-CLI consumer needs them.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

One deterministic core method, one CLI command, one cron-job example. The core reuses the change feed, hybrid retrieval, and the embedder; it adds a polarity lexicon and a pair filter, nothing heavier.

## The candidate signal

A contradiction candidate is an ordered pair `(A, B)` of chunks from two different notes. `A` is an assertion-bearing chunk from the recently-changed side; `B` is a same-subject chunk recalled from the rest of the vault. The pair is kept only when every conservative gate passes.

### 1. A-side: scope to the change feed

By default, the A-side is the set of notes changed since a stored cursor `last_contradiction_sha`, computed with `gitx.DiffNames` exactly as `service.Digest` computes its changed set. This is the highest-value framing and the cheapest: a contradiction becomes visible the moment a new statement is written that opposes an old one, and the candidate pool is bounded by recent activity, not vault size. A `--since <sha>` flag overrides the cursor; `--advance` moves the cursor to HEAD after a run, mirroring digest; `--all` sweeps every note as the A-side for a one-off full audit (off by default, because it is O(notes) recalls).

The cursor is contradiction-specific, not the digest cursor, so a digest run advancing `last_digest_sha` never blinds the contradiction scan.

### 2. Assertion-bearing filter

Not every changed line is worth checking. Restrict A-side chunks to those whose text carries an assertion or a reversal, matched by a small curated lexicon (extends the `temporal` package, which already owns the commitment regex):

- Reversal cues: `not`, `no longer`, `never`, `isn't` / `aren't` / `won't` / `can't` / `don't`, `deprecated`, `reverted`, `rolled back`, `cancelled`, `abandoned`, `dropped`, `removed`, `obsolete`, `superseded`, `replaced by`, `instead of`, `decided against`, `changed our mind`.
- Decision or assertion markers: `decided`, `we will`, `chose`, `locked`, `canonical`, `must`, `always`, `default`.

A chunk that matches neither is not an anchor and is skipped. This keeps the pool to statements that actually claim something.

### 3. B-side: recall same-subject chunks via existing retrieval

For each A-side anchor chunk, recall its nearest same-subject chunks by running the retrieval Stardust already owns, seeded with the anchor's salient terms (the content tokens the FTS tokenizer already extracts, top-k by frequency). This is `index.Store.Hybrid` unchanged: hybrid-semantic when the embedder is up (cosine plus BM25, RRF-fused), FTS-only when it is not. The recall inherits `retrieval_mode` and its degrade reason for free, satisfying goal 6. No new retrieval code, no new index.

### 4. The pair gates (the precision spine)

A recalled pair `(A, B)` becomes a candidate only when all hold:

- Different notes. `B`'s path differs from `A`'s, and neither is a parent-or-child section of the other.
- Similarity floor. The fused score of the pair is at or above a conservative threshold (default high, tuned in review), so the two are demonstrably about the same subject. In FTS-only mode this is a BM25-rank floor; in hybrid mode it is the RRF-fused score.
- Shared-term floor. `A` and `B` share content terms above a Jaccard floor over non-stopword tokens. Embedding similarity alone pairs notes that are topically near but about different subjects; the lexical overlap floor forces a shared subject and kills that class of false pair.
- Polarity asymmetry. Exactly one of `{A, B}` carries a reversal cue (XOR). Two statements that both assert, or both negate, are agreement or restatement, not contradiction. The asymmetry is the single discriminating signal, and it is deliberately narrow.
- Benign exclusions. Drop the pair when `B` supersedes `A` through the sanctioned frontmatter mechanism (`superseded_by`, `valid_to`, SPEC 12.3), when either side is a directory index or a template, and when the two notes are the same collection record versioned over time. Superseding is changing your mind on purpose, not a contradiction.

### 5. Rank, cap, dedupe

Score each surviving pair by combining the similarity and the overlap, rank descending, dedupe symmetric duplicates so `(A, B)` and `(B, A)` count once, and cap hard to a small top-N (default in the low tens, with a per-anchor cap of a few). The cap is a precision instrument: the surface must stay short enough that a human skims it, and a run that would emit hundreds is a run whose thresholds are wrong.

## The surface: `service.Contradictions` and `stardust contradictions`

One core method `service.Contradictions(ctx, opts)` returns a typed result: a slice of `Candidate{NoteA, LineA, NoteB, LineB, Score, SharedTerms, Cue, RetrievalMode, RetrievalReason}` plus rendered markdown. Each rendered candidate is an explicit review prompt with hedge, in the drift idiom:

> Possible contradiction (review): `decisions/db.md` says "we will use Postgres" and `notes/switch.md` says "no longer using Postgres". Shared terms: postgres, database. This is a candidate, not a verdict, and is likely benign; confirm before acting.

`stardust contradictions` exposes it with `--since`, `--advance`, `--all`, `--limit`, and `--output auto|md|json|plain`, matching the digest command surface. The agent calls it over Bash, as the librarian calls `stardust digest`. It is not a `check` kind and is not injected into `digest` output by default, so the integrity check stays CI-gateable and clean and the morning digest stays uncluttered.

## The agent half: a contradiction-scan cron job

A new example `docs/examples/cron-jobs/contradiction-scan/` in the librarian pattern:

- `prompt.md`: run `stardust contradictions`, and for each candidate pull both notes (`stardust query` or a note read), judge with the LLM whether the two statements genuinely conflict, and for confirmed conflicts write a short entry to `proposals/<date>-contradictions.md` under the notify / question / review taxonomy. Never edit human notes; files are the source of truth; prefer add-only. The binary prepared the candidates; the agent renders the verdict.
- `config.toml`: a scheduled `kind = "agent"` job in a read-only sandbox, mirroring the librarian's cadence.

This is the whole LLM half: a prompt file plus a config, no binary inference. It respects the charter line exactly, moving the semantic judgment into the agent dispatch where an LLM already runs.

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Judge contradictions in the binary with a rule engine or an entailment classifier. Rejected: contradiction is a semantic call; a deterministic judge either underfits (misses real conflicts) or overfits into false accusations, and a bundled classifier is the model runtime the charter forbids. The split keeps the binary deterministic and pushes the verdict to the agent.
- Make contradiction candidates a `check` kind, like drift. Rejected: `check` is integrity (broken links, malformed frontmatter, drift) and is CI-gateable through the baseline ratchet (ADR 0019). Contradiction candidates are a fuzzier, higher-false-positive surface that must never gate a commit; mixing them into `check` pollutes the ratchet and erodes trust in the whole check surface.
- Inject candidates into the morning digest by default. Rejected for v1: the digest is consumed as-is every morning and must stay clean; a noisy pair list dilutes it. The scan reuses the digest change feed but lives behind its own command, and folding a summary line into digest stays an additive future option.
- Whole-vault pairwise similarity scan. Rejected: O(notes squared) and a false-positive machine, because high similarity is agreement far more often than contradiction. Scoping the A-side to the change feed and recalling the B-side through existing retrieval bounds the work and raises precision.
- Similarity alone as the signal. Rejected: two notes that agree on a topic are highly similar; similarity is necessary but nowhere near sufficient. The polarity asymmetry is the discriminating signal, and the shared-term floor keeps the pair on one subject.
- A dedicated per-note contradiction vector or a trained model. Rejected: new dependency and new index, for a feature whose value is a short review list. Reuse of hybrid retrieval plus a lexicon is enough for v1.
- A lexical-only whole-vault negation match when the embedder is down. Deferred: without embeddings, same-subject recall falls back to FTS, which the design already does through `Hybrid`; a separate lexical-only sweep over the whole vault would be noisier than the FTS-seeded recall and is not worth the code. When the embedder is absent and FTS recall is thin, the surface returns fewer candidates and says so, rather than inventing a lower-quality signal.
- Expose contradictions over rpc and MCP in v1. Deferred: the agent reads the CLI over Bash like the librarian does, so no non-CLI consumer exists yet. The additive rpc method and MCP tool follow the parity pattern (one core, many transports) when one appears.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- False accusations, the primary failure mode. A candidate on two compatible notes trains the user to ignore the surface. Mitigation: the conjunction of a high similarity floor, a shared-term floor, a strict polarity XOR, benign-superseded exclusion, and a hard cap; plus review-prompt framing that never asserts a verdict; plus the agent as a second, semantic filter before anything reaches a proposal.
- Threshold miscalibration. Thresholds set too loose flood the list; too tight and nothing surfaces. Mitigation: precision-first defaults (high floors, low cap), tuned in the review task against real vault pairs, with the cap as a hard ceiling so a mis-tune fails short, not long.
- Negation scope is subtle. "We should not ship on Friday" versus "ship on Friday" hinges on scope the binary cannot parse. Mitigation: the binary does not try; polarity asymmetry only nominates a pair, and the agent's LLM resolves scope. A nominated non-contradiction costs one agent judgment, not a false verdict.
- Embedder-down quality drop. FTS-only recall finds fewer same-subject B-side chunks, so real contradictions can be missed. Mitigation: loud degradation (announce the mode and reason); the scan is advisory and rerunnable when the embedder returns, so a miss is recoverable, unlike a false accusation.
- Cursor coupling. Sharing the digest cursor would blind the scan after a digest advance. Mitigation: a contradiction-specific cursor `last_contradiction_sha`, advanced only by `contradictions --advance`.
- Lexicon drift and language coverage. A fixed English lexicon misses phrasings and other languages. Mitigation: v1 targets the vault's working language; the lexicon is a small, reviewable list extendable without a redesign, and a miss is a recall gap, not a false positive.

</details>

<details>
<summary><b>Open questions</b></summary>
<br>

1. Similarity floor value and whether it differs between hybrid and FTS-only modes. Default: start high in both, tune down only if recall is visibly starved in the review task.
2. Shared-term Jaccard floor and stopword list source. Default: reuse the FTS tokenizer's tokenization and a small built-in stopword set; tune the floor in review.
3. Cap defaults: total top-N and per-anchor cap. Default: low tens total, a few per anchor; revisit once run on a real vault.
4. Whether `--all` should require an explicit confirmation given its cost. Default: gate it behind the flag only, document the cost, no interactive prompt.
5. Whether a one-line "N contradiction candidates, run `stardust contradictions`" pointer belongs in digest output. Default: no in v1; keep digest clean, revisit as an additive line.
6. Lexicon extensibility: hardcoded versus a config-loaded list. Default: hardcoded small list for v1; promote to config only if the vault needs domain terms.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

End to end, adversarially, precision-first:

- Real conflict fires. Two notes, one asserting a decision and a later one reversing it over the same subject, produce exactly one candidate naming both lines, the shared terms, and the cue.
- Agreement does not fire. Two notes that agree on the same subject (both assert, high similarity) produce no candidate, proving similarity alone is inert without polarity asymmetry.
- Off-subject similarity does not fire. Two topically near notes about different subjects, one carrying a reversal cue, fall below the shared-term floor and produce no candidate.
- Superseded is excluded. A note that supersedes another through `superseded_by` or `valid_to` produces no candidate, honoring the sanctioned change-your-mind path.
- Cap holds. A constructed vault that would emit hundreds of pairs returns at most the top-N, ranked, deduped so `(A, B)` and `(B, A)` appear once.
- Degradation is loud. With the embedder up, a scan reports `retrieval_mode: hybrid-semantic`; with it down, the same scan reports `fts-only` with a reason, never silently.
- Cursor independence. A `digest --advance` between two contradiction scans does not change what the second scan sees; only `contradictions --advance` moves its cursor.
- Not a check kind. `stardust check` output is byte-identical before and after the feature; contradictions never appear there and never affect its exit code.
- Framing. Every rendered candidate is phrased as a review prompt with an explicit "candidate, not a verdict, likely benign" hedge; no output asserts a contradiction as fact.
- Agent workflow. The contradiction-scan prompt reads candidates through the CLI, writes proposals under notify / question / review, and never edits a human note.
- Gates. `go build ./...`, `go test ./...`, `make lint` clean, `gofmt -l .` empty, no U+2014 or U+2013 anywhere, `stardust check` exit 0.

</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

The rpc method and MCP tool are deferred; the typed `Candidate` result is designed so those transports inherit it additively when a non-CLI consumer appears (parity by construction, the established pattern). The lexical-only whole-vault fallback, the digest pointer line, and a config-loaded lexicon are deferred with reasons above. Judging contradictions in the binary, auto-resolving them, and any model runtime in the binary are permanently out of scope by charter.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

Feeds the plan at `docs/plans/2026-07-02-2234-cross-note-contradiction-candidates.md`. Two tasks.

- Task 1 (build): the polarity lexicon and anchor detection in `internal/temporal`, the `service.Contradictions` core method reusing the change feed and hybrid retrieval, the `stardust contradictions` CLI command, and the `contradiction-scan` cron-job example (prompt plus config) in the librarian pattern.
- Task 2 (review): the adversarial precision review that proves agreement and off-subject and superseded pairs do not fire, the cap holds, degradation is loud, the check surface is untouched, and the gate is green.

</details>

<details>
<summary><b>References</b></summary>
<br>

- SPEC.md section 11 (future work: "LLM-based contradiction detection"), section 12.4 (temporal behavior 3: "contradiction detection across notes"), section 8 ("this contradicts that"), section 12.3 (add-only, invalidate-not-delete, `superseded_by` / `valid_to`).
- ADR 0043 (this work), ADR 0016 (vectors on by default, loud degradation), ADR 0018 (drift as a review prompt), ADR 0019 (CI baseline ratchet).
- Source, verified: `internal/service/digest.go:28-108` (cursor plus `DiffNames` change feed, `temporal.Commitments` body scan), `internal/temporal/temporal.go:18-43` (commitment regex, `TopArea`), `internal/index/search.go:36-199` (`Hybrid`, FTS-only degrade, `Nearest`, RRF), `internal/embed/ollama.go:37-100` (`Available`, batch `Embed`), `internal/service/check.go:14-208` (typed `Issue`, drift as `warn`), `internal/gitx/gitx.go:41-249` (`DiffNames`, `HeadSHA`, `LastCommit`, `CommitCountSince`).
- `docs/examples/cron-jobs/librarian/prompt.md` and `config.toml` (propose-not-mutate, notify / question / review, read-only sandbox).
- `CLAUDE.md`, `.claude/rules/go.md`, `SPEC.md`.

</details>
