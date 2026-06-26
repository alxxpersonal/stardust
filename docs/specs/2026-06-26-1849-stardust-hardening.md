---
title: Stardust hardening for docs, index, links, and authoring commands
status: Draft
version: 1
date: 2026-06-26
related:
  - internal/cli/registry.go
  - internal/service/registry.go
  - internal/service/index.go
  - internal/vault/vault.go
  - internal/convention/check.go
  - plugin/claude/commands/spec.md
  - plugin/claude/commands/plan.md
  - plugin/claude/commands/doc.md
  - plugin/claude/commands/adr.md
  - docs/adr/0020-authoring-commands-reference-canonical-forge-skills.md
  - docs/adr/0021-authoring-commands-delegate-never-reimplement.md
---

Stardust MUST stop producing confident derived state from stale inputs and MUST make its plugin authoring commands run the full docs workflow inline.

<details>
<summary><b>Problem</b></summary>
<br>

Five small gaps compound into drift:

1. `stardust registry` trusts indexed collection rows. An empty or stale SQLite cache can render `docs/INDEX.md` with "No documents" while docs exist on disk.
2. `internal/vault/vault.go` extracts wikilinks from raw markdown. Links inside inline code or fenced code become graph edges and broken-link errors.
3. Incremental indexing handles explicit delete paths from git diff, but a rename can leave an old indexed row until `stardust rebuild`.
4. The docs convention says one canonical collection per doc, but the checker does not reject markdown placed in mirror folders or loose in `docs/`.
5. The Claude plugin authoring commands route to `/spec-forge` and `/doc-forge` as a second step. The required behavior is full inline baking: the command prompt itself embeds the workflow and writes docs in the same turn.

</details>

<details>
<summary><b>Context and background</b></summary>
<br>

`internal/service/registry.go` builds registry groups by calling `ListRecords` for each configured collection. The list path reads the derived index through `internal/index/records.go`. That is correct only when the index matches disk.

`internal/service/index.go` computes a catalog map, then indexes only paths from a full scan or `git diff`. It deletes rows only when a processed path no longer exists. A stale row that is absent from the current candidate path set survives.

`internal/vault/vault.go` uses `wikilinkRe` on note bodies and uses `inlineCodeRe` to find inline path refs. That makes code examples part of the graph. ADR 0015 intentionally made inline path refs first-class, but it did not require links inside code fences or format examples to become wikilinks.

`internal/convention/check.go` recognizes only `docs/specs/`, `docs/plans/`, `docs/adr/`, and `docs/research/` as governed docs. Other markdown under `docs/` is scanned for forbidden dash characters but not rejected as stray.

The plugin command files currently say they never write docs and end with `/spec-forge` or `/doc-forge` handoff lines. ADR 0020 and ADR 0021 chose that delegation. The new requirement supersedes those decisions.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

- Registry generation MUST fail loudly with the hint `index looks empty or stale, run stardust index` when indexed rows do not cover docs on disk.
- Markdown extraction MUST ignore wikilinks and inline path refs inside inline code spans and fenced code blocks, while preserving prose links.
- Incremental indexing MUST prune indexed paths that no longer exist on disk during the same pass.
- Strict docs checks MUST error on markdown under `docs/` outside registered collection folders, except `docs/INDEX.md` and `docs/templates/`.
- Plugin authoring commands MUST embed the full spec-forge or doc-forge workflow inline, gain `Write`, keep resolve-root graceful degradation, and remove all `/spec-forge` and `/doc-forge` handoff lines.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No new storage engine or migration is required.
- No `go install`, commit, or push is part of this work.
- No change to the public collection schema format.
- No automatic registry rebuild from `stardust registry`; this spec chooses a loud failure for stale registry inputs.
- No wiring to the unrelated Microsoft `.docx` `doc` skill.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

Registry stale detection:

- Add a service helper that compares registered collection folders to current markdown files on disk.
- Treat the index as stale when a configured collection has markdown files on disk but zero indexed records, or when any collection file on disk is absent from the indexed record set.
- Return a wrapped error whose message includes `index looks empty or stale, run stardust index`.
- Keep empty configured collections valid when no markdown exists on disk.

Markdown code masking:

- Add a scanner in `internal/vault/vault.go` that returns a body of the same length with inline code spans and fenced code blocks replaced by spaces.
- Fences start on a line with up to three leading spaces followed by at least three backticks or tildes and close on a matching marker of at least that length.
- Inline code spans use matching runs of backticks and honor escaped backticks.
- `ExtractLinks` and wikilink extraction in `ExtractEdges` run on the masked body.
- Inline path refs run on masked code spans derived from prose only, so code fences and code-formatted wikilink examples do not produce path refs.

Incremental index prune:

- After loading `Catalog`, scan disk once for current markdown files.
- Build a set of current paths after ignore filtering.
- Delete any catalog path missing from that set before processing candidate paths.
- Continue indexing candidate paths normally. A rename therefore deletes the old row and upserts the new one.

Stray docs:

- Load collection configs with `collections.Load`.
- Build allowed docs folder prefixes from their configured paths.
- For every markdown path under `docs/`, allow `docs/INDEX.md` and anything under `docs/templates/`.
- If no allowed prefix owns the path, emit an error issue `stray-doc`.

Inline plugin commands:

- Replace the four command bodies with embedded, executable instructions copied from the canonical skill processes, adjusted to the command mapping.
- `/stardust:spec` and `/stardust:plan` embed the complete spec-forge flow.
- `/stardust:doc` and `/stardust:adr` embed the complete doc-forge flow, with `/stardust:adr` defaulting type to `adr`.
- Set `allowed-tools: Bash, Read, Write`.
- Preserve the `resolve-root.sh` precondition and the no-workspace graceful stop.
- Update the plugin README and plugin metadata to state that commands now author docs directly.

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Auto-run `Index` from `Registry`. Rejected for now because registry should not unexpectedly mutate the index on a docs-only render command. A loud hint is safer and easier to reason about.
- Strip code blocks with regexes only. Rejected because escaped backticks, variable-length backtick spans, and tilde fences need scanner state.
- Prune only paths observed in git diff. Rejected because the bug is exactly stale indexed paths missing from the candidate set.
- Keep authoring commands as routers and install a second-step command. Rejected by the new requirement and superseded by ADR 0024.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- Registry stale checks can be too strict if collection configs point outside `docs/`. The check uses configured collection paths, not hardcoded docs folders.
- Markdown masking can accidentally hide prose after an unclosed inline span. The scanner only masks a span when it finds a closing delimiter.
- Full disk scan during incremental index adds overhead. Vault scale is local-personal, and the scan is bounded by markdown paths.
- Inline command bodies can drift from future skill changes. That is accepted by ADR 0024 because the command is now the shipped workflow.

</details>

<details>
<summary><b>Open questions</b></summary>
<br>

- Should a future `stardust registry --fix-index` auto-index before rendering? This spec leaves that out.
- Should stray docs be baseline-gated for existing large vaults? This task requires strict errors immediately.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- Add focused red tests for stale registry, code-blind link extraction, rename pruning, stray docs, and plugin command handoff removal.
- Run each focused package test before and after implementation.
- Run `stardust registry` after docs are written.
- Run full gates: `go build ./...`, `go test ./... -race -count=1`, `go vet ./...`, `gofmt -l .`, `golangci-lint run ./...`, unicode dash grep, and `stardust serve --mcp` initialize.

</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- Rewriting `docs/INDEX.md` format.
- Adding new MCP methods.
- Creating or syncing `docs/superpowers/`.
- Publishing the plugin.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. Write failing tests for stale registry detection.
2. Write failing tests for markdown code masking.
3. Write failing tests for incremental rename pruning.
4. Write failing tests for stray doc errors.
5. Write grep-backed tests for inline command baking and metadata wording.
6. Implement the smallest code changes to pass each test.
7. Regenerate the docs registry and run every gate.

</details>

<details>
<summary><b>References</b></summary>
<br>

- `/Users/alxx/.claude/skills/spec-forge/SKILL.md`
- `/Users/alxx/.claude/skills/doc-forge/SKILL.md`
- `docs/specs/2026-06-26-0418-plugin-authoring-commands.md`
- `docs/adr/0020-authoring-commands-reference-canonical-forge-skills.md`
- `docs/adr/0021-authoring-commands-delegate-never-reimplement.md`
- `docs/adr/0022-docs-plans-canonical-native-plan-mirrors.md`

</details>
