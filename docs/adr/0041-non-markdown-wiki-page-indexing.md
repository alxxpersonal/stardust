---
title: Non-markdown wiki pages are indexed as title plus plain text, not rendered
status: Accepted
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-1759-non-markdown-wiki-page-indexing.md
  - docs/plans/2026-07-02-1759-non-markdown-wiki-page-indexing.md
  - docs/research/2026-06-27-1721-github-wiki-compatibility.md
  - internal/vault/vault.go
  - internal/vault/chunk.go
  - internal/graph/graph.go
  - internal/convention/check.go
---

# Non-markdown wiki pages are indexed as title plus plain text, not rendered

Stardust indexes the non-markdown formats GitHub wikis render (`.rst`, `.adoc`, `.textile`, `.org`, and the rest) by extracting a title and a cheap plain-text body with pure-Go line rules. It does not parse or render them faithfully. The formats are recognized everywhere, not gated to a detected wiki. Non-markdown pages are indexed, searchable, and resolvable link targets, but they do not emit graph edges or participate in drift.

## Context

The GitHub wiki compatibility research (`docs/research/2026-06-27-1721-github-wiki-compatibility.md`) closed eight of nine improvements. Improvement 9, non-markdown page support, is the last open proposal. GitHub picks a wiki page's renderer by file extension and the `github/markup` library renders nine formats; Stardust's `vault.Scan` filters to `.md` / `.markdown` only, so every other page is dropped before parsing and is invisible to index, search, bundle, and graph.

Three facts constrain the design. First, Stardust's job on a wiki is indexing (title, prose text for FTS and vectors, resolvable link targets), not rendering. Second, the index path is tolerant: SQLite FTS5 tokenizes on non-alphanumeric boundaries, so markup punctuation mostly vanishes at query time, and embeddings tolerate residual markup. Third, GitHub derives a wiki page's title from its filename, so a de-slugified filename is always an authoritative title fallback. Together these mean the high-value signal (a good title plus prose words) is cheap to extract, and faithful parsing buys little.

The hard constraints are explicit: no heavyweight parser or rendering dependency, and markdown behavior (`.md` / `.markdown` indexing, linking, graphing, rendering) must stay byte-identical, with non-markdown support purely additive.

## Decision

Index non-markdown wiki pages additively through the existing pipeline, with cheap per-format extraction, and lock the scope tightly.

- **Recognize the `github/markup` non-markdown extensions in `Scan`, everywhere.** Add `.adoc`, `.asciidoc`, `.rst`, `.rest`, `.textile`, `.org`, `.creole`, `.mediawiki`, `.wiki`, `.rdoc`, `.pod`, `.pod6`. Exclude `.asc` (collides with PGP key and signature files). No gating on directory `Kind`: `Scan` is `Kind`-unaware and shared by six callers, threading `Kind` in to gate is a fork-like change, and these are prose documentation formats worth indexing wherever they appear. The `ignore` config is the existing escape hatch.
- **`Parse` dispatches on extension; markdown is untouched.** `.md` / `.markdown` keep the current code path verbatim. A supported non-markdown extension routes to a new `parseNonMarkdown` helper that sets `Title` and a plain-text `Body`, leaving `Frontmatter`, `Tags`, and `Links` nil. `Hash` is unchanged so reindex still skips by content hash.
- **Two extraction tiers, by marginal value.** Tier A (AsciiDoc, reStructuredText, Textile, Org) gets a small per-format title heuristic (first heading in the format's syntax) plus a line-rule body reducer that strips heading markers and drops comment and directive lines. Tier B (Creole, MediaWiki, RDoc, Pod) gets a filename title and the raw body text. Both tiers are fully indexed and searchable; the tiers differ only in title fidelity and light body cleanup. All extraction is regex and line rules, no parser.
- **No links, no drift, from non-markdown pages.** `ExtractEdges` and `CodeRefs` short-circuit to empty for a non-markdown note. These pages emit no wikilink, markdown-link, `related`, or inline-path edges and bind no code.
- **Non-markdown pages are resolvable link targets.** Generalize `trimMarkdownExtension` (used only by `GraphKey` normalization and relative-link normalization) to strip the non-markdown extensions too, so a markdown `[[Page]]` resolves to `Page.rst`. Markdown keying is byte-identical because the `.md` / `.markdown` cases and the extension-less default are unchanged.
- **Chunking and rendering are unchanged.** Non-markdown bodies flow through `Chunks` as one heading-less section (windowed by the existing `splitOversize` when large) and render through glamour as plain paragraphs. No per-format chunker, no per-format renderer.
- **`check` applies the forbidden-dash rule to non-markdown pages and nothing else.** The docs-convention block (stray-doc, doc-name, schema, `related`, `governs`, drift) is guarded to markdown files.

## Consequences

- The last open wiki-compatibility proposal is closed for search: `.rst`, `.adoc`, `.textile`, `.org`, and the rest are indexed, returned by `query`, present in the registry and bundle, and resolvable as `[[Page]]` targets, killing the md-to-non-md broken-link false positive.
- Markdown behavior is byte-identical. The markdown `Parse` branch, `Chunks`, `ExtractEdges`, `GraphKey`, and rendering are unchanged; the full existing test suite is the regression proof.
- Non-markdown pages are second-class only where it is cheap to be so: no heading-aware chunks, no faithful render, no outbound edges, no drift. Each is a deliberate, reversible deferral, not a dead end.
- A code repo with vendored non-markdown docs will index them; the `ignore` config is the control, matching how `.md` is already indexed everywhere.
- The scope is small and additive: one extension set, one `Parse` dispatch, one new helper file, one extension-trim generalization, one `check` guard. No new dependency, no config schema change.

## Alternatives considered

- **Full format-aware parsing via real libraries** for faithful headings, rendered-to-text bodies, and link graphs. Rejected. Several formats lack a maintained pure-Go parser (implying CGO or a subprocess), it violates the "index, don't render" scope, and the FTS and vector quality gain over cheap plain text is low because tokenization already discards markup and embeddings tolerate it.
- **Per-format heading-aware chunking.** Deferred. `splitOversize` already keeps large non-markdown pages searchable; faithful per-format heading detection is the expensive part for a marginal retrieval gain.
- **Per-format outbound links and drift.** Rejected for v1. The formats' link syntaxes target URLs, anchors, and section IDs more than page files, so cheap extraction would manufacture false edges; and these formats carry no `related` / `governs` frontmatter to bind drift against.
- **Gate the new extensions to a detected wiki `Kind`.** Rejected. `Scan` is `Kind`-unaware and shared by six callers; gating is a fork-like change for prose formats worth indexing anywhere, and `ignore` already lets a user exclude them.
- **Filename title for every format (Tier B for all).** Rejected. AsciiDoc, RST, Textile, and Org are common enough and their title rules cheap enough that filename-only titles would visibly degrade results for the formats users actually have; Tier A spends about ten lines per format for a real gain and stops there.
- **Index `.asc` as AsciiDoc.** Rejected. `.asc` is predominantly a PGP armored file; indexing keys and signatures as prose is pure noise for negligible AsciiDoc coverage.

## References

- docs/research/2026-06-27-1721-github-wiki-compatibility.md (improvement 9, "Left as proposals")
- docs/specs/2026-07-02-1759-non-markdown-wiki-page-indexing.md
- `internal/vault/vault.go` (`Scan`, `Parse`, `ExtractEdges`, `CodeRefs`, `GraphKey`, `trimMarkdownExtension`)
- `internal/vault/chunk.go` (`Chunks`, `splitOversize`)
- `internal/graph/graph.go` (`resolveIndex`, `byBase` basename resolution)
- `internal/convention/check.go` (`CheckDocs`, the forbidden-dash and docs-convention blocks)
- GitHub Markup supported formats and extensions: github/markup
