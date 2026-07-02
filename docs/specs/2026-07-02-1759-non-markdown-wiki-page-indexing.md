---
title: Index non-markdown GitHub wiki pages for search
status: Draft
version: 1
date: 2026-07-02
related:
  - docs/adr/0041-non-markdown-wiki-page-indexing.md
  - docs/plans/2026-07-02-1759-non-markdown-wiki-page-indexing.md
  - docs/research/2026-06-27-1721-github-wiki-compatibility.md
  - internal/vault/vault.go
  - internal/vault/chunk.go
  - internal/graph/graph.go
  - internal/convention/check.go
  - internal/service/index.go
---

# Index non-markdown GitHub wiki pages for search

GitHub wikis render nine markup formats, but Stardust only scans `.md` / `.markdown`, so `.rst`, `.adoc`, `.textile`, and the rest are invisible to index, search, bundle, and graph. Make those pages first-class index and search citizens with cheap, pure-Go, per-format title and plain-text extraction. No new parser dependencies. Markdown behavior stays byte-identical; non-markdown support is purely additive.

<details>
<summary><b>Problem</b></summary>
<br>

The GitHub wiki compatibility research (`docs/research/2026-06-27-1721-github-wiki-compatibility.md`) shipped slug resolution, pipe fallback, structural-page exemptions, docs-convention gating, and cross-repo drift, closing eight of nine improvements. Improvement 9, "Support non-Markdown wiki pages deliberately," is the last open proposal.

GitHub documents that a wiki page's renderer is chosen by file extension, and the open-source `github/markup` library renders nine formats: Markdown, AsciiDoc, reStructuredText, Textile, Org, Creole, MediaWiki, RDoc, and Pod. Stardust's `vault.Scan` filters the walk to `.md` and `.markdown` only (`internal/vault/vault.go`, the extension test in `Scan`). Every other wiki page is dropped before parsing, so it never reaches the FTS index, the vector store, the registry, the bundle context, or the link graph. On a real wiki that mixes formats, a `.rst` install guide or an `.adoc` handbook simply does not exist as far as `stardust query` is concerned, and a markdown page that links to it looks like a broken link.

The research already flagged the caveat in its capability table: "Indexing and hybrid search - Works with caveats - non-Markdown wiki formats are ignored." This spec removes that caveat for search, and makes non-markdown pages resolvable link targets, without pulling in a rendering toolchain.

</details>

<details>
<summary><b>Context</b></summary>
<br>

The pipeline these pages must join, and the exact seams:

- `internal/vault/vault.go`, `Scan(root, ignore)`: a pure `filepath.WalkDir` that returns slash-relative paths whose lowercased extension is `.md` or `.markdown`, minus ignored dirs and the generated registry. It has no knowledge of directory `Kind`; it is called from six sites (`graph.Build`, `convention.CheckDocs`, `service.Index`, `service.Registry`, `service.GetNote`'s link resolver, `service.Governs`).
- `internal/vault/vault.go`, `Parse(root, rel)`: reads the file, parses YAML frontmatter, sets `Title` (frontmatter `title`, else first `# H1`, else filename minus `.md`), `Body` (source after the frontmatter block), `Tags` (frontmatter plus inline `#hashtags`), `Links` (normalized `[[wikilinks]]`), and `Hash`.
- `internal/vault/chunk.go`, `Chunks(note)`: splits `note.Body` on markdown headings (`^#{1,6}\s+`), carries the note title and tags into each chunk, and hard-splits oversized sections into overlapping windows via `splitOversize`. `ChunkEmbedText` embeds `title + heading + body`.
- `internal/vault/vault.go`, `ExtractEdges(root, note)` and `CodeRefs(root, note)`: derive graph edges (wikilinks, markdown links, `related` frontmatter, inline repo paths) and the doc-to-code binding set drift watches. Both read `note.Body` and stat the real filesystem.
- `internal/vault/vault.go`, `GraphKey(rel)` -> `normalizePagePath` -> `trimMarkdownExtension`: the node-key normalizer. `trimMarkdownExtension` strips only `.md` / `.markdown`, so a non-md path keeps its extension in its key. The graph resolver (`internal/graph/graph.go`, `resolveIndex`) matches an unqualified target against `idx.keys[target]` and a unique `byBase` basename, so a leftover extension in the key blocks a markdown `[[Page]]` from resolving to `Page.rst`.
- `internal/render/glamour.go`, `GlamourRender`: renders a note body as markdown for the CLI and TUI. It is display-only and never feeds the index.
- `internal/convention/check.go`, `CheckDocs`: scans, runs the forbidden-dash rule on every file, then, only when the docs convention is active (a code repo or committed collections), applies stray-doc, doc-name, schema, `related`, `governs`, and drift checks. The doc-name and doc-type gates are already `.md`-anchored regexes.
- `internal/index/*`, `internal/manifest/*`: the FTS5 + vector store keys notes by full path and stores whatever `Title` and chunks it is handed; `manifest.WriteIndex` renders `Title` and `Path`. Neither assumes a `.md` extension.

Two facts from the research fix the extraction floor. First, GitHub derives a wiki page's title from its filename, not its content, so the filename (de-slugified) is always an authoritative title fallback. Second, GitHub warns page titles away from `\ / : * ? " < > |`, so wiki filenames are clean slugs.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. `Scan` returns the supported non-markdown wiki page formats so they reach `Parse` and the index.
2. `Parse` yields a usable `Title` and a plain-text `Body` for each supported non-markdown format, using only small pure-Go line and regex rules, no parser dependency.
3. Non-markdown pages are indexed (FTS + vectors), searchable (`query`), and present in the registry and bundle context, exactly like markdown notes.
4. Non-markdown pages are resolvable link targets: a markdown `[[Page]]` that points at `Page.rst` resolves, so it stops being a false broken link.
5. Markdown parsing, chunking, linking, graphing, and rendering are byte-identical to today. Every non-markdown behavior is additive.
6. The forbidden-dash rule extends to non-markdown pages; no other docs-convention or drift rule applies to them.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No faithful rendering of non-markdown formats. Stardust indexes; it does not become a markup renderer. Non-markdown bodies render through the existing glamour path as plain paragraphs, which is acceptable and unchanged surface.
- No per-format heading-aware chunking in v1. Non-markdown bodies flow through `Chunks` as a single heading-less section, hard-split by the existing `splitOversize` window when large. Faithful heading boundaries per format are deferred.
- No outbound link extraction from non-markdown pages, and no drift participation. These pages are link sinks: resolvable targets with no out edges and no doc-to-code bindings.
- No new parser or rendering dependency. No `asciidoctor`, no docutils bridge, no CGO. Extraction is regex and line rules only.
- No markdown-alias expansion. GitHub's markdown extensions include `.mdown`, `.mkdn`, and more; Stardust deliberately keeps `.md` / `.markdown`. That is a separate decision, out of scope here.
- No config schema change and no gating on directory `Kind`. The new extensions are recognized wherever they appear (see Approach).

</details>

<details>
<summary><b>Approach</b></summary>
<br>

One principle: the markdown path is untouched; non-markdown is a parallel, additive branch that produces a `Note` the rest of the pipeline already knows how to index.

**Extension set (allow everywhere).** Extend the `Scan` filter to also accept the `github/markup` non-markdown extensions:

```
.adoc .asciidoc      (AsciiDoc)
.rst .rest           (reStructuredText)
.textile             (Textile)
.org                 (Org)
.creole              (Creole)
.mediawiki .wiki     (MediaWiki)
.rdoc                (RDoc)
.pod .pod6           (Pod)
```

`.asc` (also an AsciiDoc extension per `github/markup`) is deliberately excluded: it collides with ASCII-armored PGP key and signature files, which would be indexed as garbage. The gain from `.asc` is negligible and the collision is real.

These are recognized everywhere, not gated to a detected wiki `Kind`. `Scan(root, ignore)` is a pure filesystem walk with no `Kind` argument, called from six sites; threading `Kind` through it to gate would be an invasive, fork-like change against the "extend, never fork" constraint. The formats are prose documentation formats, so indexing them wherever they appear matches Stardust's "index docs, not code" mission (a Sphinx `.rst` tree or an AsciiDoc handbook in a code repo is genuinely searchable documentation). Markdown behavior is identical regardless of `Kind`, and a user who wants to exclude a format uses the existing `ignore` config.

**Parse dispatch.** `Parse` dispatches on the lowercased extension. `.md` / `.markdown` take the current code path unchanged. A supported non-markdown extension routes to a new `parseNonMarkdown(rel, raw)` helper (new file `internal/vault/wikimarkup.go`) that returns a `Note` with:

- `Title`: the format's title heuristic (Tier A) or the filename fallback (Tier B and every fallback), where the filename fallback is `filepath.Base(rel)` minus its extension. This mirrors GitHub, which titles a wiki page from its filename.
- `Body`: a plain-text reduction of the source (Tier A) or the raw source text (Tier B).
- `Frontmatter`: nil. These formats have no YAML frontmatter convention.
- `Tags`: nil. Inline `#hashtag` scanning assumes markdown; `#` is meaningful in several of these formats.
- `Links`: nil. See link participation below.
- `Hash`: `ContentHash(raw)`, unchanged, so reindex still skips unchanged pages by hash.

**Per-format extraction tiers (YAGNI split).**

| Format | Ext | Tier | Title rule | Body |
|---|---|---|---|---|
| AsciiDoc | `.adoc` `.asciidoc` | A | first `= Title` line (single `=` + space) | line-reduced |
| reStructuredText | `.rst` `.rest` | A | first text line immediately followed by an adornment line (a run of one punctuation char at least as long) | line-reduced |
| Textile | `.textile` | A | first `hN. ` line's text | line-reduced |
| Org | `.org` | A | `#+TITLE:` value, else first `* ` headline | line-reduced |
| Creole | `.creole` | B | filename | raw text |
| MediaWiki | `.mediawiki` `.wiki` | B | filename | raw text |
| RDoc | `.rdoc` | B | filename | raw text |
| Pod | `.pod` `.pod6` | B | filename | raw text |

Tier A gets a small per-format reducer because these four formats show up most in real wikis and docs trees and have a dead-simple, unambiguous heading rule that a few regexes clean up well. Tier B gets filename title plus raw body: those formats are rarer, and their markup either tokenizes cleanly under FTS anyway (MediaWiki, Creole wiki punctuation) or is niche (RDoc, Pod), so a bespoke stripper is not worth the maintenance. Both tiers are fully indexed and fully searchable; the only difference is title fidelity and light body cleanup.

**Tier A body reduction (cheap line rules, no parser).** The reducer's job is narrow: get clean prose words for FTS and embeddings and a clean title. SQLite FTS5 tokenizes on non-alphanumeric boundaries, so residual inline punctuation barely affects search; the high-value operations are (1) capture the title, (2) strip heading markers so heading text survives as prose, (3) drop obvious non-prose lines. Minimal per-format rules:

- AsciiDoc: drop comment lines (`//`) and attribute-entry lines (`:name:`); blank block-delimiter lines (`----`, `====`, `....`, `****`, `|===`, `--`); strip a leading `=`+`space` run from heading lines; collapse inline `*` `_` `` ` `` markers.
- reStructuredText: drop directive and comment lines (`.. `); drop adornment-only lines (a line that is a single punctuation char repeated); strip inline `` `` ``, `*`, `**`, and role `:name:` markers; reduce `` `text <url>`_ `` to `text`.
- Textile: strip a leading `hN. ` / `bq. ` / `p. ` signature; strip inline `*` `_` `@` markers; reduce `"text":url` to `text`.
- Org: drop `#+...` keyword lines, `:PROPERTIES:` / `:END:` drawers, and `#+BEGIN`/`#+END` block markers; strip a leading `*`+`space` run from headlines; strip `/ * = ~ _` emphasis; reduce `[[link][desc]]` to `desc` and `[[link]]` to `link`.

Rules are the smallest set that yields clean prose; residual punctuation is left to FTS tokenization rather than chased.

**Chunking (no change).** The reduced or raw non-markdown body flows through `Chunks` unchanged. With no `#` markdown headings it becomes one section, then `splitOversize` windows it if large. Small wiki pages become one chunk. This keeps `chunk.go` untouched and byte-identical for markdown.

**Graph participation: resolvable target nodes, no out edges.**

- Node key: generalize `trimMarkdownExtension` (used only by `normalizePagePath` -> `GraphKey` and relative-link normalization) to also strip the supported non-markdown extensions, so `GraphKey("Install.rst")` yields `install`, matching how a markdown `[[Install]]` normalizes. Markdown keying is byte-identical: the `.md` / `.markdown` cases are unchanged, and only non-markdown segments (which previously fell through unstripped and only ever occur for these new pages) gain stripping. This lets an inbound `[[Page]]` from a markdown note resolve to `Page.rst`, removing the md-to-non-md broken-link false positive, consistent with the whole wiki-compatibility thrust.
- Out edges: `ExtractEdges` and `CodeRefs` short-circuit to empty for a non-markdown note (guarded on the note's extension). A non-markdown page emits no wikilink, markdown-link, `related`, or inline-path edges, so it never manufactures a graph edge or a drift binding from arbitrary prose tokens.
- Net: non-markdown pages are link sinks. They receive inbound edges and resolve as targets, but originate none. Markdown-syntax relative links `[x](foo.rst)` are still not treated as page edges (`markdownPageTarget` keeps rejecting non-md extensions); only extension-less `[[wikilinks]]` resolve to non-markdown nodes.

**Convention check (dash-only for non-markdown).** In `CheckDocs`, the forbidden-dash rule continues to run on every scanned file, now including non-markdown pages, consistent with the repo-wide no-dash rule. Guard the docs-convention block (stray-doc, doc-type, schema, `related`, `governs`, drift) to markdown files only, so a non-markdown file under `docs/` in a code repo is not newly flagged and drift never runs on a non-markdown page.

**Renderer (no change).** Non-markdown bodies render through `GlamourRender` as plain paragraphs. No per-format renderer, no special-casing. Display-only, low-stakes, deferred.

</details>

<details>
<summary><b>Alternatives</b></summary>
<br>

- **Full format-aware parsing via real libraries** (an AsciiDoc toolchain, a docutils-equivalent, Textile/Org parsers) to get faithful headings, rendered-HTML-to-text bodies, and reliable link graphs. Rejected. Several formats have no maintained pure-Go parser (some imply CGO or a subprocess), it violates the "index, don't render" scope, and the marginal FTS and vector quality over cheap plain-text extraction is low: FTS tokenization already discards markup punctuation and embeddings tolerate it. The signal that matters, title plus prose words, is captured by cheap rules.
- **Per-format heading-aware chunking.** Translate each format's heading syntax into `#` chunk boundaries so large pages chunk by section. Deferred. `splitOversize` already keeps large non-markdown pages searchable; faithful per-format heading detection is the expensive part for a marginal retrieval gain. Revisit if large non-markdown pages prove to retrieve poorly.
- **Per-format outbound link and drift participation.** Extract each format's link syntax (`link:` / `xref:` / `` `x <url>`_ `` / `"x":url` / `[[x][y]]`) as graph edges and honor a drift binding. Rejected for v1. These syntaxes mostly target URLs, anchors, and section IDs rather than page files; the page-targeting subset is small and unreliable to isolate cheaply, so extracting them risks false edges. Non-markdown formats also have no `related` / `governs` frontmatter, so there is nothing to bind drift against.
- **Gate the new extensions to a detected wiki `Kind`.** Recognize non-markdown pages only when `DetectKind` returns `KindGitHubWiki`. Rejected. `Scan` is `Kind`-unaware and shared by six callers; threading `Kind` in is a fork-like change for a formats set that is prose documentation worth indexing anywhere. The `ignore` config is the existing per-user escape hatch.
- **Index the raw bytes for every format with no title heuristic** (Tier B for all). Simpler, but AsciiDoc, RST, Textile, and Org are common enough and their title rules cheap enough that filename-only titles would visibly degrade result quality for the formats users actually have. The Tier A split spends about ten lines per format for a real gain and stops there.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- **Noise in code repos.** A code repo with a large vendored `.rst` tree gets those pages indexed. Mitigation: the existing `ignore` config excludes directories; the default ignore already covers `node_modules`, and users add `vendor` or `.venv` as needed. Consistent with how `.md` is already indexed everywhere.
- **Key collision.** `Page.md` and `Page.rst` in one wiki both key to `page`; the graph node is last-writer-wins. Rare (GitHub itself would treat both as the same title) and low-impact: both are still indexed and searchable because the index catalog keys by full path. Documented, not fixed.
- **Orphan warnings.** A non-markdown page nothing links to is reported as an orphan, exactly like an unlinked markdown note. Structural pages (`_Sidebar`, `_Footer`, `Home`) are already exempt. This is a warning, not an error, and consistent behavior.
- **Dash rule on prose.** Em and en dashes are normal in general prose, and the forbidden-dash rule now flags them in non-markdown pages. This is intentional parity with markdown, where the rule already applies to prose; it is a warning-level surface via `check`.
- **Reducer imperfection.** The cheap reducers will leave some markup artifacts or over-strip an edge case. Acceptable: the body feeds FTS and embeddings, both tolerant of residual or missing punctuation, and the title has a filename fallback. No correctness path depends on perfect reduction.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- Unit (`internal/vault`): `Scan` returns each new extension and still excludes unrelated files and the generated registry; `.asc` is not returned. `Parse` on a fixture per format yields the expected Tier A title (and filename fallback when the heuristic misses) and a body with heading markers and comment/directive lines removed; Tier B yields filename title and raw body. `Parse` on `.md` / `.markdown` fixtures is unchanged (existing tests stay green verbatim).
- Unit: `trimMarkdownExtension` strips each supported non-markdown extension and is unchanged for `.md` / `.markdown` and for extension-less segments; `GraphKey("Install.rst") == "install"`.
- Unit: `ExtractEdges` and `CodeRefs` return empty for a non-markdown note even when its body contains `dir/dir/file.ext` tokens and pseudo-links.
- Graph: a fixture vault with `Home.md` linking `[[Install]]` and an `Install.rst` file resolves the edge (no broken link), and `Install.rst` has zero out edges.
- Index: indexing a mixed-format fixture indexes the non-markdown pages (catalog count includes them), `query` returns a hit whose body text came from a non-markdown page, and a reindex with no change skips them by hash.
- Convention: `CheckDocs` flags a forbidden dash inside a non-markdown page; a non-markdown file under `docs/` in a docs-convention repo is not flagged stray and no drift runs on it; markdown docs-convention results are unchanged.
- Regression: the full existing suite passes unchanged, proving markdown behavior is byte-identical.
- Gate: `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` empty, zero U+2014 / U+2013, `stardust check` exit 0.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. `internal/vault`: add the supported-extension set and route `Scan` and `Parse` through it; add `internal/vault/wikimarkup.go` with the Tier A title heuristics, the Tier A reducers, the Tier B raw path, and the filename fallback; unit-test per format.
2. `internal/vault`: generalize `trimMarkdownExtension` to strip the non-markdown extensions; short-circuit `ExtractEdges` and `CodeRefs` for non-markdown notes; unit-test keying and the empty-edge guarantee.
3. `internal/convention/check.go`: guard the docs-convention block to markdown files while keeping the dash rule global; test the dash-on-non-markdown and no-drift-on-non-markdown cases.
4. Integration: a mixed-format fixture vault proves index, query, registry, bundle, and graph resolution; assert the markdown suite is unchanged.
5. Mark research improvement 9 and the "Left as proposals" non-markdown line shipped; regenerate the docs index.

</details>
