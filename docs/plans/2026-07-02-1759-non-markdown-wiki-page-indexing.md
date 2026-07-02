---
title: Non-markdown wiki page indexing - implementation plan
status: Active
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-1759-non-markdown-wiki-page-indexing.md
  - docs/adr/0041-non-markdown-wiki-page-indexing.md
  - docs/research/2026-06-27-1721-github-wiki-compatibility.md
  - internal/vault/vault.go
  - internal/vault/chunk.go
  - internal/graph/graph.go
  - internal/convention/check.go
  - internal/service/index.go
---

# Non-markdown wiki page indexing - implementation plan

Make the `github/markup` non-markdown wiki formats (`.rst`, `.adoc`, `.textile`, `.org`, and the rest) visible to Stardust's index and search with cheap, pure-Go, per-format title and plain-text extraction. Markdown behavior stays byte-identical; non-markdown support is additive. Implements spec `2026-07-02-1759-non-markdown-wiki-page-indexing.md` and ADR 0041.

## Header

- **Goal:** non-markdown wiki pages are indexed, searchable, present in registry and bundle, and resolvable as `[[Page]]` link targets, with no new parser dependency and no markdown regression.
- **Architecture:** extend `vault.Scan` and `vault.Parse` with a supported-extension set and a `parseNonMarkdown` helper (new `internal/vault/wikimarkup.go`); generalize `trimMarkdownExtension` for node keying; short-circuit `ExtractEdges` / `CodeRefs` for non-markdown; guard the docs-convention block in `CheckDocs` to markdown while keeping the dash rule global. Everything else in the pipeline (chunker, index store, manifest, renderer) is unchanged.
- **Tech stack:** Go 1.26, `regexp` / `strings` / `path/filepath` (all already imported in `internal/vault`). No new dependencies.
- **Global constraints:** conventional commits, imperative lowercase, no co-author or generated-by trailers, `go build ./...` + `go test ./...` + `make lint` green, `gofmt -l .` empty, `stardust check` exit 0, ZERO U+2014 / U+2013.

## Context

The GitHub wiki compatibility research (`docs/research/2026-06-27-1721-github-wiki-compatibility.md`) left improvement 9, non-markdown page support, as the last open proposal. GitHub picks a wiki page's renderer by extension; Stardust's `Scan` filters to `.md` / `.markdown`, so every other page is dropped before parsing. This plan makes those pages index and search citizens and resolvable link targets, using only regex and line rules, per ADR 0041's title-plus-plain-text decision.

## Reuse map (read first)

- `internal/vault/vault.go` - `Scan` (the extension filter to extend), `Parse` (the dispatch point), `ExtractEdges` / `CodeRefs` (short-circuit for non-markdown), `GraphKey` -> `normalizePagePath` -> `trimMarkdownExtension` (the key normalizer to generalize), `ContentHash`, `Note`.
- `internal/vault/chunk.go` - `Chunks`, `splitOversize` (consumed unchanged; non-markdown body flows through as a heading-less section).
- `internal/graph/graph.go` - `resolveIndex`, `byBase` basename resolution (why extension-stripped keys matter for `[[Page]]` -> `Page.rst`).
- `internal/convention/check.go` - `CheckDocs` (the forbidden-dash loop stays global; the docs-convention block gets a markdown guard).
- `internal/service/index.go` - `Index` (the indexing driver: `Scan` -> `Parse` -> `Chunks` -> `UpsertNote`, consumed unchanged).
- `internal/index/*`, `internal/manifest/*` - path-keyed store and `WriteIndex`; consumed unchanged, no `.md` assumption.

## Task 1: build non-markdown scan, parse, keying, and convention guard

- Modify: `internal/vault/vault.go`, `internal/convention/check.go`
- Add: `internal/vault/wikimarkup.go`
- Test: `internal/vault/vault_test.go`, `internal/vault/wikimarkup_test.go`, `internal/graph/graph_test.go`, `internal/convention/check_test.go`

- [ ] Add the supported non-markdown extension set (a package-level map or switch): `.adoc`, `.asciidoc`, `.rst`, `.rest`, `.textile`, `.org`, `.creole`, `.mediawiki`, `.wiki`, `.rdoc`, `.pod`, `.pod6`. Add an `isMarkdownExt` / `isSupportedPageExt` predicate pair. Explicitly exclude `.asc`.
- [ ] Extend `Scan`: accept `.md`, `.markdown`, and any supported non-markdown extension; keep the generated-registry skip and ignore-dir behavior unchanged.
- [ ] Add `internal/vault/wikimarkup.go` with `parseNonMarkdown(rel string, raw []byte) Note`: sets `Hash = ContentHash(raw)`, `Title` (Tier A heuristic or filename fallback), `Body` (Tier A reduced text or Tier B raw text), and leaves `Frontmatter` / `Tags` / `Links` nil.
- [ ] Tier A title heuristics: AsciiDoc first `= Title` line; reStructuredText first text line immediately followed by an adornment line (one punctuation char repeated, length at least the text length); Textile first `hN. ` line text; Org `#+TITLE:` value else first `* ` headline. Filename fallback (`filepath.Base(rel)` minus its extension) for a miss and for all Tier B formats.
- [ ] Tier A body reducers (small regex / line rules, no parser): strip heading markers, drop comment and directive and block-delimiter lines, collapse inline emphasis punctuation per the spec's per-format rule list (AsciiDoc, RST, Textile, Org). Tier B body is the raw source text unchanged.
- [ ] Route `Parse` to dispatch on extension: `.md` / `.markdown` take the existing code path verbatim (do not refactor it); a supported non-markdown extension returns `parseNonMarkdown`.
- [ ] Generalize `trimMarkdownExtension` to also strip the supported non-markdown extensions; confirm the `.md` / `.markdown` and extension-less cases are byte-identical.
- [ ] Short-circuit `ExtractEdges` and `CodeRefs` to return empty for a non-markdown note (guard on `note.Path`'s extension).
- [ ] `internal/convention/check.go`: in `CheckDocs`, keep the forbidden-dash check running on every scanned file; guard the docs-convention block (stray-doc, doc-type, schema, `related`, `governs`, drift) so it runs only for markdown files.
- [ ] Unit-test: `Scan` returns each new extension and excludes `.asc`; `Parse` per-format title and body (Tier A reduced, Tier B raw, filename fallback); `.md` / `.markdown` parse unchanged; `trimMarkdownExtension` / `GraphKey("Install.rst") == "install"`; `ExtractEdges` / `CodeRefs` empty for a non-markdown note whose body contains path tokens and pseudo-links; graph resolves `[[Install]]` -> `Install.rst` with zero out edges on the target; `CheckDocs` flags a dash in a non-markdown page and runs no drift on it. Run, loop to green.
- [ ] `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` green.
- [ ] Commit `feat(vault): index non-markdown wiki page formats for search`.

## Task 2: integration, review, document, and gate

- Modify: `docs/research/2026-06-27-1721-github-wiki-compatibility.md`, this plan, the spec
- Add: `internal/service/index_test.go` coverage (or extend existing) for a mixed-format fixture
- Verify: full gate and self-review

- [ ] Integration: a mixed-format fixture vault (`Home.md`, `Install.rst`, `Guide.adoc`, a `.textile` and `.org` page) indexes the non-markdown pages (catalog count includes them), `query` returns a hit whose body came from a non-markdown page, a no-change reindex skips them by hash, and `[[Install]]` from `Home.md` resolves. Run, loop to green.
- [ ] Self-review against ADR 0041: markdown `Parse` / `Chunks` / `GraphKey` / rendering untouched; non-markdown emits no out edges and no drift; extensions recognized everywhere; `.asc` excluded; no new dependency.
- [ ] Confirm the full existing suite passes unchanged as the markdown byte-identical proof.
- [ ] Mark research improvement 9 and the "Left as proposals" non-markdown line shipped, referencing this spec and ADR 0041.
- [ ] Set the spec `status` to `Implemented` and this plan `status` to `Done`.
- [ ] Regenerate the docs index: `stardust index && stardust registry`.
- [ ] Full gate: `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` empty, zero U+2014 / U+2013 in touched files, `stardust check` exit 0.
- [ ] Commit `docs(vault): mark non-markdown wiki page indexing shipped`.

## Verification

- Scan and parse: every supported extension is scanned, `.asc` is not; each format yields a title and a plain-text body; markdown parsing is unchanged.
- Keying and edges: non-markdown node keys are extension-stripped so `[[Page]]` resolves to `Page.rst`; non-markdown notes emit zero out edges and zero code refs.
- Index and query: non-markdown pages are indexed, returned by `query`, present in registry and bundle, and skipped by hash on a clean reindex.
- Convention: the dash rule flags non-markdown pages; no docs-convention or drift rule runs on them; markdown results are unchanged.
- `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` green; `stardust check` exit 0; zero em or en dashes.

## Self-review gate

- Every spec Work-breakdown item maps to a step here.
- The markdown `Parse`, `Chunks`, `GraphKey`, and render paths are untouched; the existing suite is the regression proof.
- Non-markdown pages are index-and-search plus resolvable target only: no out edges, no drift.
- Extensions are recognized everywhere, not gated to a wiki `Kind`; `.asc` is excluded.
- No new parser or rendering dependency; extraction is regex and line rules only.
