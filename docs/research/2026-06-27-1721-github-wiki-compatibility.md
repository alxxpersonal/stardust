---
title: GitHub Wiki Compatibility
type: research
status: Active
date: 2026-06-27
created: 2026-06-27
updated: 2026-06-27
---

Thesis: Stardust is a strong fit for GitHub wiki search and health checks, but it needed wiki slug resolution, plain-frontmatter tolerance, and structural-page handling before `check` could be trusted on a real wiki.

<details>
<summary>GitHub wiki format facts</summary>

GitHub wikis are separate Git repositories. GitHub documents the clone URL as `https://github.com/YOUR-USERNAME/YOUR-REPOSITORY.wiki.git`, and local edits are ordinary commits to that wiki repository. Source: [GitHub Docs: adding or editing wiki pages](https://docs.github.com/en/communities/documenting-your-project-with-wikis/adding-or-editing-wiki-pages).

Page content is file-backed. GitHub says the filename determines the page title and the extension determines the renderer. Markdown files such as `foo.md` and `foo.markdown` use the Markdown converter. GitHub also warns against page-title characters that produce bad cross-platform filenames: `\ / : * ? " < > |`. Source: [GitHub Docs: about wiki filenames](https://docs.github.com/en/communities/documenting-your-project-with-wikis/adding-or-editing-wiki-pages#about-wiki-filenames).

The space-to-hyphen page slug rule comes from the Gollum wiki link model that GitHub wikis historically follow. Gollum documents `[[Frodo Baggins]]` as linking to a page file named `Frodo-Baggins.ext`, with spaces converted to hyphens. It also documents slash-to-hyphen behavior for page-link tags and a breadth-first search for page files. Source: [Gollum README via RubyDoc](https://www.rubydoc.info/gems/gollum/2.4.3).

Pipe wikilinks in Gollum put display text first and target second: `[[Frodo|Frodo Baggins]]` displays `Frodo` and links to `Frodo-Baggins.ext`. Obsidian uses the reverse convention, so a compatibility layer should treat the second side as a fallback, not a replacement. Source: [Gollum README via RubyDoc](https://www.rubydoc.info/gems/gollum/2.4.3).

GitHub accepts Markdown links and relative links in rendered Markdown. That makes `[Label](Page-Name)` and file-style links relevant for a wiki checker even though they are not `[[wikilinks]]`. Source: [GitHub Docs: basic writing and formatting syntax](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax).

Special wiki pages are filename conventions. GitHub uses `_Footer.<extension>` and `_Sidebar.<extension>` to populate shared wiki chrome. `Home.md` is the normal landing page in cloned GitHub wikis. Sources: [GitHub Docs: footer or sidebar](https://docs.github.com/en/communities/documenting-your-project-with-wikis/creating-a-footer-or-sidebar-for-your-wiki), plus a live clone of `adam-p/markdown-here.wiki.git` showing `Home.md` and hyphenated page files.

GitHub documents wikis as content written like elsewhere on GitHub, rendered through the open-source Markup library, and notes some wiki-specific Markdown differences such as footnotes not being supported in wikis. There is no GitHub wiki YAML frontmatter convention; title and renderer come from filename and extension. Sources: [GitHub Docs: about wikis](https://docs.github.com/en/communities/documenting-your-project-with-wikis/about-wikis), [GitHub Docs: formatting syntax](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax).

GitHub documents a soft limit of 5,000 total wiki files. That is still well inside Stardust's local SQLite and flat-vector design for personal or project docs, but it matters for scan and graph UX. Source: [GitHub Docs: about wikis](https://docs.github.com/en/communities/documenting-your-project-with-wikis/about-wikis).

</details>

<details>
<summary>Stardust capability comparison</summary>

| Capability | Pre-patch verdict on a GitHub wiki | Why | Post-patch verdict |
|---|---|---|---|
| Indexing and hybrid search | Works with caveats | `vault.Scan` recursively indexes `.md`; `vault.Parse` falls back to filename title; `index.Hybrid` searches FTS5 plus vectors. Caveat: non-Markdown wiki formats are ignored. | Works with same caveats |
| Query JSON and headless use | Works | `query --output json` emits JSON hits and the output layer keeps ANSI out of JSON. | Works |
| Link graph for `[[Page Name]]` to `Page-Name.md` | Breaks | `NormalizeLink` lowercased but kept spaces, while file keys for `Page-Name.md` became `page-name`. `[[Page Name]]` became `page name`, so graph and check saw a broken link. | Implemented fix |
| Link graph for `[[Title|Page Name]]` | Breaks or caveat | Existing parsing treated the left side as target, matching Obsidian but not Gollum. | Implemented fallback: try Obsidian target first, then Gollum target |
| Broken-link detection | Breaks for spaced wiki links | Broken links are derived from unresolved graph edges, so slug mismatch caused false errors. | Works for wikilinks after slug fix; Markdown relative links remain a proposal |
| Orphan detection | Caveat | `_Sidebar.md`, `_Footer.md`, and `Home.md` could be flagged as orphans even though GitHub uses them structurally. | Implemented special-page exemption |
| Frontmatter and docs conventions | Breaks in plain wiki repos with `docs/` | `CheckDocs` enforced docs/specs/plans/adr/research naming and frontmatter for any matching path, even when `init` would classify the directory as a plain vault. | Implemented docs-convention gating |
| Collections as structured records | Caveat | Works only when the user explicitly registers collections. Plain wiki pages have `{}` frontmatter and are not records. | Same |
| Doc-code drift engine | Mostly not applicable | A standalone wiki repo is separate from source, so `governs` and inline code refs usually cannot bind to real source paths. | Implemented for `governs:` through explicit `source_root` |
| Special pages in PageRank | Caveat | Structural pages still exist as graph nodes. That is acceptable for search and graph context, but they should not become cleanup warnings. | Orphan warning fixed |
| Init behavior | Works | `init` already detects markdown-dominant repos as plain vaults and skips docs collections unless `--docs` is passed. | Preserved |

</details>

<details>
<summary>Prioritized improvements</summary>

1. Implement GitHub wiki slug-aware wikilink resolution.
   - Change: let graph and note resolution treat `Page-Name.md` as a match for `[[Page Name]]`, case-insensitively within Stardust's existing normalized key model.
   - Value: removes the largest false-positive class in `stardust check`.
   - Effort: small.
   - Status: implemented.

2. Support Gollum pipe fallback without breaking Obsidian.
   - Change: keep Obsidian target-before-pipe as primary, but when it does not resolve, try the target-after-pipe candidate.
   - Value: real GitHub wiki custom-label links stop looking broken.
   - Effort: small.
   - Status: implemented.

3. Gate docs-convention checks to docs-convention repos.
   - Change: only enforce docs folder schemas and stray-doc rules when `.stardust/collections` opts in or `DetectKind` sees a code repo, matching `init`.
   - Value: plain wiki pages no longer need YAML frontmatter or timestamped filenames.
   - Effort: small.
   - Status: implemented.

4. Exempt GitHub structural pages from orphan warnings.
   - Change: skip `_Sidebar.md`, `_Footer.md`, and `Home.md` in `Graph.Orphans`.
   - Value: cleaner wiki health output.
   - Effort: small.
   - Status: implemented.

5. Parse Markdown relative links as graph edges.
   - Change: extract `[text](Page-Name)`, `[text](Page-Name.md)`, `./Page-Name`, and `../Page-Name` when they point to Markdown pages in the wiki repo; skip URLs, anchors, images, and static assets.
   - Value: GitHub wikis often use Markdown links as much as `[[wikilinks]]`.
   - Effort: medium.
   - Status: proposal.

6. Add explicit wiki detection.
   - Change: detect a remote ending in `.wiki.git`, or add `kind = "github-wiki"` to config for local clones without remotes.
   - Value: lets future checks tune behavior without guessing from file mix.
   - Effort: medium.
   - Status: proposal.

7. Make subdirectory page resolution more faithful.
   - Change: resolve same-basename pages by relative path where possible, then unique global slug, and report ambiguity clearly.
   - Value: reduces surprises for folderized wikis.
   - Effort: medium.
   - Status: proposal.

8. Add a wiki-code coherence variant.
   - Change: allow a wiki page to declare source bindings such as `governs` or a lightweight comment marker, with paths resolved against a sibling source checkout or configured root.
   - Value: brings Stardust's strongest angle to wikis: "this page claims to document this code, and the code moved."
   - Effort: larger, needs product design.
   - Status: implemented for explicit `source_root` plus `governs:`.

9. Support non-Markdown wiki pages deliberately.
   - Change: decide whether to index `.rst`, `.textile`, `.adoc`, and other GitHub Markup formats as plain text or with format-aware conversion.
   - Value: broader wiki coverage.
   - Effort: medium to large.
   - Status: proposal.

</details>

<details>
<summary>Implemented in this pass</summary>

Implemented:

- `internal/vault`: added wikilink candidate extraction, skipped external `[[http...]]` links, added GitHub wiki display aliases, and added a title-check option for plain vaults.
- `internal/graph`: added slug alias resolution, Gollum pipe fallback, and structural-page orphan filtering.
- `internal/convention`: added docs-convention activation based on committed collections or code-repo detection.
- `internal/service`: made `check` use filename-title tolerance for plain vaults and made `get_note` resolve wiki slug candidates.
- Tests: added focused coverage for wiki slug resolution, pipe fallback, structural pages, plain wiki checks, and docs-convention gating.
- Follow-up pass: added Markdown relative links as graph edges, explicit `github-wiki` detection, path-aware foldered wiki resolution, and same-repo wiki or vault `governs` drift bindings.
- Follow-up pass: added explicit `source_root` config for cross-repo wiki-to-code drift. When a `governs:` path is absent from the wiki checkout and present under the configured source repo, Stardust counts source repo commits after the wiki page's last commit time and labels the binding as source repo drift.

Left as proposals:

- Auto-detecting a sibling source checkout for `.wiki.git` clones.
- Non-Markdown wiki format indexing.

</details>

<details>
<summary>Risk notes</summary>

- Pipe syntax is inherently ambiguous between Obsidian and Gollum. The implementation keeps Obsidian primary and uses the Gollum side as fallback, which is conservative for existing vaults.
- Stardust still lowercases link keys. Live GitHub URL checks accepted lower-case wiki page URLs for a public wiki, and current Stardust behavior already depended on lowercasing. This is a compatible tolerance, not a claim that every underlying Git operation is case-insensitive.
- Relative Markdown links are now checked when they target Markdown wiki pages. External URLs, anchors, images, and static assets are skipped.
- Structural pages are only exempt from orphan reporting. They remain indexed and can still participate in search and PageRank.
- Same-repo `governs` bindings work for wiki and vault pages. A standalone `.wiki.git` repo can now govern a sibling source checkout when `.stardust/config.toml` sets `source_root`; empty `source_root` preserves same-repo behavior.

</details>
