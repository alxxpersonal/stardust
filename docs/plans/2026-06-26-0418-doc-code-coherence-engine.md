---
title: Doc-code coherence engine
status: Done
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-doc-code-coherence-engine.md
  - docs/adr/0014-single-declarative-collection-schema.md
  - docs/adr/0015-related-and-inline-refs-as-edges.md
  - docs/adr/0016-vectors-on-by-default-loud-degradation.md
  - docs/adr/0017-git-derived-created-updated.md
  - docs/adr/0018-drift-detection-by-commit-distance.md
  - docs/adr/0019-ci-baseline-ratchet.md
  - docs/adr/0023-collection-scoped-link-namespacing.md
---

# Doc-code coherence engine

Transform Stardust from an indexer plus frontmatter linter into a doc-code coherence engine by closing three structural defects (Tier 1), making docs trustworthy (Tier 2), and shipping adoption polish (Tier 3), with the drift-detection moat built last.

## Goal

Keep docs true to the code as the code moves underneath them. The implementation ladders to that: a single schema both consumers obey, a graph that sees every edge, semantic retrieval that is on and honest, git-derived dates, a remediating `--fix`, drift detection bound to referenced code, CI ratcheting, scoped link names, bundle provenance, and a freshness stamp.

## Architecture

Pure-Go single binary. Markdown plus frontmatter plus git are the source of truth; the SQLite index (FTS5 plus brute-force cosine vectors), link graph, and embeddings are disposable caches rebuilt by `stardust rebuild`. Git is the change feed and the temporal ground truth. Service methods return typed results rendered to `--output json|md`. No cgo, no C extensions, no network vector store.

## Tech stack

Go 1.26.1, module `github.com/alxxpersonal/stardust`. `cmd/stardust/main.go` thin, `internal/` only. cobra plus fang CLI, `pelletier/go-toml/v2`, `modernc.org/sqlite` plus `pressly/goose`, charm v2 TUI, Ollama `bge-m3` embeddings, optional cross-encoder reranker over HTTP.

## Global constraints

- `.claude/rules/go.md`: goimports grouping, `// --- Section ---` separators, doc comments on exports starting with the name, third-person verbs, `json` tags on API types, lowercase error messages prefixed with operation context, `%w` wrapping (never `%v`), `errors.Is` for sentinels, never panic, no em or en dashes (U+2014, U+2013) anywhere.
- Content hash, never mtime, is the index authority.
- Graceful degradation at every layer: no Ollama and no reranker yields an honest FTS engine, announced.
- Conventional commits, no co-author tags. Commit only when alxx asks.

## Context

The defects, verified in source:
- Schema lives in three disagreeing places: `service/docs.go:renderNewDoc` (99-117) emits 5 fields, `convention/check.go:checkDocFile` (line 103) hardcodes the same 5, `.stardust/collections/specs/config.toml` and `cli/init.go:docCollectionConfig` (122-133) declare 2. `collections.Validate(fm, fields)` (`collections/collections.go:130`) already exists, unused by the checker.
- Edges: `vault/vault.go:ExtractLinks` (45-58) is wikilink-only; `graph/graph.go:Build` (line 50) sets `Out: note.Links`; `convention/check.go:checkRelated` (124-140) validates `related:` but discards it.
- Vectors: no config field; runtime-gated on `s.embed.Available` (`service/index.go:51`); silent mid-run degrade (`index.go:79`); note-level hash skip (`index.go:69`); reranker unwired (`config.go:34` empty default).
- Drift exists but gated: `service/governs.go:annotateStaleness` (192-213) and `check.go:checkStale` (169-180) need `governs:` plus `status: Implemented`.

## Reuse map (read these first, confirm real signatures in source)

- `internal/collections/collections.go`: `Field`, `Config`, `LoadOne(dir, name)`, `Validate(fm map[string]any, fields []Field) error`, `ErrValidation`.
- `internal/convention/convention.go`: `DocCollection{Name,Path,Description,Statuses}`, `DefaultDocCollections()`, `StringList(fm, key)`, `DocStatusAllowed`.
- `internal/convention/check.go`: `CheckDocs`, `checkDocFile`, `checkRelated`, `checkGoverns`, `checkStale`, `docTypeForPath`, `DocTypeForPath`.
- `internal/vault/vault.go`: `Note`, `ExtractLinks`, `NormalizeLink`, `Parse`, `ContentHash`, `Scan`.
- `internal/graph/graph.go`: `Node{Path,Title,Out,In}`, `Build`, `Orphans`, `BrokenLinks`, `Neighbors`, `PersonalizedPageRank`.
- `internal/service/index.go`: `Index`, `embedChunks`, content-hash skip at line 69, degrade at 79, `last_indexed_sha` at 90.
- `internal/service/checkfix.go`: `CheckFix`, `fixableKinds`, `fixDocFields`, `fileDate`.
- `internal/service/governs.go`: `annotateStaleness`, `GovernedDoc`, `StaleDocs`, `GoverningDocs`.
- `internal/service/bundle.go`: `BundleItem`, `Bundle`, `packBundle`.
- `internal/gitx/gitx.go`: `IsRepo`, `HeadSHA`, `LastCommit`, `CommitCountSince`, `DiffNames`, `run`.
- `internal/index/search.go`: `Hybrid`, `searchFTS`, `searchVec`, `rrfK`.
- `internal/rerank/rerank.go`: `Client.Rerank`.

## Execution rules

Mirror these tasks into the harness todo tool. Keep exactly one task in progress at a time. Flip each step's box the moment it is done (`[ ]` idle, `[wip]` in progress, `[x]` complete, `[f]` failed), never batch the ticks. Do not exit a task until its tests pass; if a command fails, fix the cause and re-run until green. Every task ends with `go build ./... && go test ./... && go vet ./... && gofmt -l .` clean and a dash scan.

---

## Phase 1: Tier 1 (structural)

### Task 1.1: One schema, two consumers (ADR 0014)

Files:
- Modify: `internal/convention/convention.go`, `internal/cli/init.go`, `internal/convention/check.go`, `internal/service/checkfix.go`
- Test: `internal/convention/convention_test.go`, `internal/convention/check_test.go`, `internal/cli/init_test.go`, `internal/service/checkfix_test.go`

Interfaces:
- Produces: `convention.DocCollection.Fields() []collections.Field` returning the ordered full field set per collection (`title` string required; `type` enum [collection-type] required; `status` enum [statuses] required; `created` date required; `updated` date required; `governs` ref optional; `related` ref optional).
- Consumes: `collections.LoadOne`, `collections.Validate`.

Steps:
- [x] Write failing test: `convention.DefaultDocCollections()[i].Fields()` yields exactly the seven fields with correct types, required flags, and enum values for specs/plans/adr/research.
- [x] Run it (red).
- [x] Implement `Fields()` on `DocCollection` (import `internal/collections`; watch for an import cycle, since `collections` must not import `convention`; if a cycle appears, define the field builder in a small leaf or return `[]collections.Field` directly without back-import).
- [x] Run it (green).
- [x] Write failing test: `docCollectionConfig(c)` emits all seven TOML field-table blocks with types and enums (parse the TOML back with `collections.LoadOne` against a temp dir and assert the field set).
- [x] Run it (red), implement `docCollectionConfig` to codegen from `c.Fields()`, run (green).
- [x] Write failing test in `check_test.go`: a doc missing `created` reports `missing-doc-field`; a doc with `status: Bogus` reports a status violation; both come through `collections.Validate`, and with no collection config present the legacy hardcoded set still fires.
- [x] Run it (red).
- [x] Implement `checkDocFile`: load the owning collection via `collections.LoadOne(s-less helper using root)`, call `collections.Validate(fm, cfg.Fields)`, map its `ErrValidation` message to a `ConventionIssue`; fall back to the current hardcoded loop only when `LoadOne` finds no config. Keep `checkRelated`, `checkGoverns`, `checkStale` calls intact.
- [x] Run it (green).
- [x] Write failing test in `checkfix_test.go`: fixability is schema-derived, where a required field with a derivation (`type`, `created`, `updated`) is fixable and `title`/`status` are not.
- [x] Run it (red), refactor `fixableKinds` into a function of the loaded schema, run (green).
- [x] Validation loop: `go build ./... && go test ./... && go vet ./... && gofmt -l .` clean, dash scan clean.

Deliverable: scaffolder, checker, and autofixer all derive field rules from the per-collection schema; a generated doc cannot fail its own linter.

### Task 1.2: Related and inline refs as edges (ADR 0015)

Files:
- Modify: `internal/vault/vault.go`, `internal/graph/graph.go`, `internal/convention/check.go`
- Test: `internal/vault/vault_test.go`, `internal/graph/graph_test.go`, `internal/service/check_test.go`

Interfaces:
- Produces: `vault.Edge{Target string; Kind string}` (`json` tags), `vault.ExtractEdges(note Note) []Edge`, and a resolver `graph` uses to classify a target as a doc node or a code reference by on-disk existence.
- Consumes: `convention.StringList(note.Frontmatter, "related")`, `vault.NormalizeLink`, `os.Stat`.

Steps:
- [x] Write failing test: `ExtractEdges` on a note with one wikilink, a `related: [docs/specs/x.md]`, and an inline ``` `internal/store/daemon.go` ``` returns three edges with kinds `wikilink`, `related`, `inline-path`; a non-resolving inline token yields no edge.
- [x] Run it (red).
- [x] Implement `ExtractEdges`: wikilinks from `wikilinkRe`; related from `StringList`; inline-path candidates from backtick spans matching a `dir/dir/file.ext` shape, kept only when the resolved repo path exists. Keep `ExtractLinks` for backward compatibility; have `Parse` also populate edges (add a `Note.Edges []Edge` field or compute in `graph.Build`). NOTE: `vault` cannot import `convention` (convention imports vault), so related extraction uses a local `fmStringList` helper rather than `convention.StringList`; `ExtractEdges(root, note)` takes root to resolve inline-path existence; edges are computed in `graph.Build`.
- [x] Run it (green).
- [x] Write failing test in `graph_test.go`: a doc reachable only by `related:` is not an orphan and is reachable by `PersonalizedPageRank`; a plain wikilink still works; an inline code path is not a graph node.
- [x] Run it (red).
- [x] Implement `graph.Build` to add typed out-edges from `ExtractEdges`, classify each target (in-vault `.md` note becomes a graph edge, resolvable non-`.md` repo file becomes a code reference stored separately for drift), and feed only doc edges into `Out`/`In`. Carry `Kind` on the node's out-edges.
- [x] Run it (green).
- [x] Write failing test in `service/check_test.go`: `checkRelated` still flags a non-existent `related:` target and the related edge participates in the graph.
- [x] Run it (red), confirm `checkRelated` unchanged for validation while edges flow, run (green).
- [x] Validation loop clean, dash scan clean.

Deliverable: the graph sees wikilink plus related edges; doc-to-code references are captured for Phase 4 drift.

### Task 1.3: Turn on the differentiator (ADR 0016)

Files:
- Modify: `internal/index/migrations/000X_chunk_hash.sql` (new), `internal/index/*` (chunk hash storage and read), `internal/service/index.go`, `internal/service/query.go` (or wherever `Query` lives), `internal/service/bundle.go`, `internal/rerank` wiring
- Test: `internal/index/*_test.go`, `internal/service/index_test.go`, `internal/service/query_test.go`

Interfaces:
- Produces: per-chunk content hash column; `retrieval_mode` field on the query and bundle result types (`hybrid-semantic` or `fts-only`) plus a degrade reason; reranker invocation over the fused top-k.
- Consumes: `vault.ContentHash`, `embed.Client.Available`, `rerank.Client.Rerank`, `Store.Hybrid`.

Steps:
- [x] Write failing migration test: a new goose migration adds a `chunk_hash` (or per-chunk hash) column and is idempotent.
- [x] Run it (red), add the migration, run (green).
- [x] Write failing test in `index_test.go`: editing one heading of a multi-chunk note re-embeds only the changed chunk (assert embed call count or changed-vector count), unchanged chunks keep their vectors.
- [x] Run it (red).
- [x] Implement per-chunk hashing in `Index`: compute a hash from the exact embedded text (`title + "\n" + heading + "\n" + body`, matching `embedChunks`), store it, and on reindex skip chunks whose hash is unchanged instead of skipping at note granularity. NOTE: `chunk_hash` is computed inside `UpsertNote` from `vault.ChunkEmbedText` (keeps the signature stable for its 8 callers); `Service.embedNoteChunks` reuses unchanged vectors via `Store.ChunkVectors`; `Service.embed` became an `embedder` interface so tests inject a fake.
- [x] Run it (green).
- [x] Write failing test in `query_test.go`: with vectors available the result carries `retrieval_mode: hybrid-semantic`; with the embedder unavailable it carries `retrieval_mode: fts-only` and a non-empty reason.
- [x] Run it (red), add `retrieval_mode` plus reason to the query and bundle result types and set it from `embed.Available` plus the actual `queryVec != nil` path, run (green).
- [x] Write failing test: when `RerankerURL` is set the fused top-k is reordered by `rerank.Client.Rerank`; when unset or unreachable the order is the fused order and `retrieval_mode` notes rerank state.
- [x] Run it (red), wire `rerank` into `Query` over the fused top-k, run (green).
- [x] Validation loop clean, dash scan clean.

Deliverable: semantic retrieval is incremental and on by default; degradation is announced; rerank is wired.

---

## Phase 2: Tier 2 (trustworthy, drift excluded)

### Task 2.1: Git-derived created and updated (ADR 0017)

Files:
- Modify: `internal/gitx/gitx.go`, `internal/service/checkfix.go`
- Test: `internal/gitx/gitx_test.go`, `internal/service/checkfix_test.go`

Interfaces:
- Produces: `gitx.FirstCommitDate(ctx, root, path string) (string, error)`, `gitx.LastCommitDate(ctx, root string, paths ...string) (string, error)`, both `YYYY-MM-DD` or empty when untracked.
- Consumes: `gitx.run`.

Steps:
- [x] Write failing test in `gitx_test.go`: build a temp git repo, commit a file twice on two dates, assert `FirstCommitDate` is the add date and `LastCommitDate` is the latest; an untracked path yields empty string, no error.
- [x] Run it (red).
- [x] Implement both via `run`: `log --diff-filter=A --follow --format=%ad --date=short -- <path>` (take the last line) and `log -1 --format=%ad --date=short -- <paths>`. Wrap errors with `%w` and operation context. NOTE: both guard on a local `hasHead` helper (rev-parse --verify HEAD) so a non-repo or unborn branch returns empty without error; `FirstCommitDate(ctx, root, path)` and `LastCommitDate(ctx, root, paths...)` keep the ADR signatures. Also added `gitx.Move` (git mv) and `gitx.IsTracked` for Task 2.2.
- [x] Run it (green).
- [x] Write failing test in `checkfix_test.go`: in a git repo, `--fix` sets `created` to the first commit date and `updated` to the last; on a non-repo or untracked file it falls back to the mtime date.
- [x] Run it (red).
- [x] Implement `fixDocFields` to prefer git dates when `gitx.IsRepo` and the file is tracked, else `fileDate`. Keep `fileDate` for the fallback. NOTE: tracked detection is implicit (git date funcs return empty when untracked, triggering the `fileDate` fallback via `docFirstDate`/`docLastDate`); fill stays gated on the field being empty (no overwrite of present dates).
- [x] Run it (green).
- [x] Validation loop clean, dash scan clean.

Deliverable: dates are a derived projection of git, reconciled by `--fix`.

### Task 2.2: check --fix as a codemod (ADR 0014 plus 0017)

Files:
- Modify: `internal/service/checkfix.go`
- Test: `internal/service/checkfix_test.go`

Interfaces:
- Produces: filename remediation for `bad-doc-name` (rename to convention via `git mv` when tracked); git-derived dates replacing mtime; existing `forbidden-dash` and `type` fixes retained.
- Consumes: `gitx.FirstCommitDate`, `convention.DocTypeForPath`, `nextADRNumber` (in `service/docs.go`), `slugify`.

Steps:
- [x] Write failing test: an off-convention filename in `docs/specs/` is renamed to `<first-commit-date>-<slug>.md` and an ADR to `<next-number>-<slug>.md`, preserving git history via `git mv` when tracked; `title` and `status` stay report-only.
- [x] Run it (red).
- [x] Implement the rename fix: derive the date prefix from `FirstCommitDate` (fallback mtime), the slug from the title for timestamped collections, the next zero-padded number plus slug for ADRs; use `git mv` when tracked, else `os.Rename` through the memory store; reindex the new path. NOTE: timestamped names take a `<first-commit-date>-0000-<slug>.md` form (the `0000` time slot satisfies `timestampedDocNameRe`, which requires HHMM, since `FirstCommitDate` is date-only); slug falls back to the filename stem then the doc type when the title is empty; `git mv`/`IsTracked` added to `gitx`.
- [x] Run it (green).
- [x] Add `bad-doc-name` to the schema-derived fixable set; ensure date fixes use git (from Task 2.1). NOTE: `bad-doc-name` is unconditionally fixable (a convention name is always derivable), so it is handled as its own `namePaths` group in `CheckFix` rather than gated through `fixableDocFields`.
- [x] Write failing test: a doc with a forbidden dash, a missing type, a missing date, and an off-convention name is fully remediated in one `--fix`, with `title`/`status` issues remaining.
- [x] Run it (red), confirm the orchestration groups all fixes per file, run (green).
- [x] Validation loop clean, dash scan clean.

Deliverable: `--fix` remediates every mechanically fixable issue; the report becomes a codemod.

---

## Phase 3: Tier 3 (adoption and polish)

### Task 3.1: CI baseline and ratchet (ADR 0019)

Files:
- Create: `internal/service/baseline.go`
- Modify: `internal/cli/check.go`, `internal/service` check surface
- Test: `internal/service/baseline_test.go`, `internal/cli/check_test.go`

Interfaces:
- Produces: `.stardust/baseline.json` schema (a list of fingerprints: kind, path, normalized detail); `stardust check --ci` (report and exit on new issues only); `stardust check --update-baseline` (snapshot).
- Consumes: `convention.CheckDocs`, `config.Layout`.

Steps:
- [x] Write failing test: a fingerprint is stable across runs for the same issue and differs across kind/path/detail.
- [x] Run it (red), implement `Fingerprint(issue)` and baseline load/save, run (green).
- [x] Write failing test: with a baseline covering all current issues, `--ci` exits zero; introduce one new issue and it exits non-zero on exactly that one; `--update-baseline` re-snapshots.
- [x] Run it (red), implement the `--ci` subtraction and exit code plus `--update-baseline`, run (green).
- [x] Validation loop clean, dash scan clean.

Deliverable: a dirty repo adopts the gate green and fails only on new errors.

### Task 3.2: Collection-scoped link resolution (ADR 0023)

Files:
- Modify: `internal/vault/vault.go`, `internal/graph/graph.go`, `internal/convention/check.go`
- Test: `internal/vault/vault_test.go`, `internal/graph/graph_test.go`

Interfaces:
- Produces: collection-qualified key resolution accepting double-bracket `specs/slug` and `spec:slug` forms, emitting path-style; deterministic resolution order (in-collection, then unique global, then warn).
- Consumes: `NormalizeLink`, `docTypeForPath`.

Steps:
- [x] Write failing test: the same slug under `specs/` and `plans/` produces two distinct graph nodes and a `specs/slug` wikilink resolves to the spec; an unqualified `slug` wikilink resolves in-collection first.
- [x] Run it (red).
- [x] Implement qualified keys beside `NormalizeLink` (do not change its basename behavior for unqualified inputs); add the resolver order and an ambiguity warning.
- [x] Run it (green).
- [x] Write failing test: the duplicate-name warning no longer fires for cross-collection slug reuse but still fires for a true in-collection duplicate.
- [x] Run it (red), adjust the duplicate detection, run (green).
- [x] Validation loop clean, dash scan clean.

Deliverable: cross-collection slugs are unambiguous; spurious duplicate-name warnings are gone.

### Task 3.3: Provenance in bundle returns (spec point 9)

Files:
- Modify: `internal/service/bundle.go`
- Test: `internal/service/bundle_test.go`

Interfaces:
- Produces: `BundleItem.Provenance []string` (`json:"provenance"`) with values `keyword`, `semantic`, `link-expansion` (carrying edge `Kind`), `frontmatter-ref`; rendered inline by `packBundle`.
- Consumes: the hybrid path (FTS vs vector origin), the PageRank path (seed plus edge `Kind` from Task 1.2).

Steps:
- [x] Write failing test: a note that is both an FTS hit and a PageRank neighbor carries both `keyword` and `link-expansion`; a `related:`-only neighbor carries `frontmatter-ref`.
- [x] Run it (red).
- [x] Thread provenance through `Bundle`: tag hybrid items by their origin and PageRank items by the edge that reached them; render in `packBundle`.
- [x] Run it (green).
- [x] Validation loop clean, dash scan clean.

Deliverable: every bundle item says why it was selected.

### Task 3.4: Live freshness stamp (spec point 10)

Files:
- Modify: `internal/service/bundle.go`, `internal/manifest` (if the manifest also stamps)
- Test: `internal/service/bundle_test.go`

Interfaces:
- Produces: a freshness line on the bundle and pack: "index is N commits behind HEAD".
- Consumes: `store.GetMeta(ctx, "last_indexed_sha")`, `gitx.HeadSHA`, `gitx.CommitCountSince`.

Steps:
- [x] Write failing test: with the index at `last_indexed_sha` and N new commits to HEAD, the bundle markdown contains "index is N commits behind HEAD"; at HEAD it says zero or omits.
- [x] Run it (red).
- [x] Compute `CommitCountSince(last_indexed_sha, HEAD)` in `Bundle` and stamp the pack header; guard non-repo and empty-cursor cases.
- [x] Run it (green).
- [x] Validation loop clean, dash scan clean.

Deliverable: a bundle declares how far behind HEAD its index is.

---

## Phase 4: The moat (drift detection, built last) (ADR 0018)

### Task 4.1: Drift detection bound to referenced code

Files:
- Modify: `internal/service/governs.go`, `internal/convention/check.go`, `internal/service/bundle.go`, `internal/manifest`
- Test: `internal/service/governs_test.go`, `internal/convention/check_test.go`, `internal/service/bundle_test.go`

Interfaces:
- Produces: drift bound to `governs:` globs, `related:` code targets, and inline code-path refs (from Task 1.2); a `drift` `warn` in `check`; drift lines in the manifest and bundle; commit count per binding.
- Consumes: `gitx.LastCommit`, `gitx.CommitCountSince`, the code references from `vault.ExtractEdges`, `annotateStaleness`.

Steps:
- [x] Write failing test: an ADR with `related:` and an inline path to a code file, with N commits to that file since the ADR's last commit, yields a `drift` warning naming the file and N; with zero commits since, no warning. A `related:`-bound doc with no `Implemented` status still drift-checks; a `governs:`-bound doc keeps its existing gating.
- [x] Run it (red).
- [x] Generalize the binding: collect code references for a doc from `ExtractEdges` (kinds `related` resolving to code, and `inline-path`) in addition to `governs:` matches; compute commit-distance per bound file with `gitx.LastCommit` plus `gitx.CommitCountSince`; ungate reference bindings from `Implemented` while leaving `governs:` gating as-is. NOTE: code references reuse the new `vault.CodeRefs(root, note)` helper (which both `convention.checkDrift` and `service.docDrift` call) and the link graph's already-classified `Node.CodeRefs`. The convention `checkDrift` emits a per-file `drift` warn; `service.DriftDocs` is the ungated, graph-keyed counterpart to `StaleDocs`; the existing `governs:`-plus-`Implemented` `checkStale`/`stale-governed-doc`/`annotateStaleness` path is untouched.
- [x] Run it (green).
- [x] Write failing test: `check` surfaces the drift `warn`, the manifest renders the drift line, and `bundle` includes drift for any bound doc it returns.
- [x] Run it (red), surface drift in all three renderers, run (green). NOTE: `check` inherits drift automatically through `convention.CheckDocs`; the manifest gains a `Docs referencing moved code` section fed by `DriftDocs`; `BundleItem` gains a `Drift []DriftBinding` field rendered inline by `packBundle`.
- [x] Write failing test: drift is phrased as a review prompt ("review"), never as an error severity, so it does not fail `--strict` by itself.
- [x] Run it (red), set severity `warn` and phrasing, run (green).
- [x] Validation loop clean, dash scan clean.

Deliverable: any doc that references moved code announces itself in `check`, the manifest, and bundles. The moat is live.

---

## Verification (whole plan)

Run from `~/Desktop/Stardust`:

- Build and gates: `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .` empty, `make lint` clean, and a repo-wide scan for U+2014 and U+2013 returns nothing.
- Schema: `stardust new spec "Probe"` then `stardust check` reports zero; remove `type` and `stardust check --fix` restores it; remove `title` and it stays reported.
- Edges: a `related:`-only doc is absent from `stardust graph orphans` and present in `stardust bundle` expansion.
- Vectors: with Ollama up, `stardust query --output json` shows `retrieval_mode: hybrid-semantic`; with Ollama down it shows `fts-only` plus a reason; a one-heading edit re-embeds one chunk.
- Git dates and fix: `stardust check --fix` sets `created`/`updated` to match `git log` and renames an off-convention file via `git mv`.
- CI: `stardust check --update-baseline` then `stardust check --ci` exits zero; add a forbidden dash and `--ci` exits non-zero on exactly that.
- Namespacing: same slug under `specs/` and `plans/` no longer warns and a `specs/slug` wikilink resolves correctly.
- Provenance and freshness: bundle items carry provenance; the pack header states commits behind HEAD.
- Drift (capstone): commit an ADR referencing `internal/store/daemon.go`, push commits to it, and `stardust check`, the manifest, and the bundle all flag drift with the commit count.

## Self-review gate

Before declaring done: re-read against the spec; confirm every spec requirement maps to a task here; confirm function and type names are consistent across tasks (`ExtractEdges`, `Edge`, `retrieval_mode`, `FirstCommitDate`, `LastCommitDate`, `Provenance`, drift `warn`); confirm no task left a box unflipped; confirm the drift moat was built after Phases 1 and 2 it depends on; run the full verification list and capture output as evidence before any success claim.
