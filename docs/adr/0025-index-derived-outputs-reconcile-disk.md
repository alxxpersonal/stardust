---
title: Index-derived outputs reconcile against disk
status: Proposed
version: 1
date: 2026-06-26
related:
  - internal/service/index.go
  - internal/service/registry.go
  - internal/index/upsert.go
  - internal/cli/registry.go
---

Derived outputs MUST reconcile the SQLite index against markdown files on disk before presenting authoritative state.

<details>
<summary><b>Context</b></summary>
<br>

Stardust treats markdown files as truth and the SQLite index as a rebuildable cache. Registry and query surfaces read the cache for speed. That cache can be empty after an upgrade or stale after a moved file.

</details>

<details>
<summary><b>Decision</b></summary>
<br>

Incremental indexing reconciles the catalog against the current markdown file set and prunes any indexed path missing on disk.

Registry generation validates that indexed collection rows cover the collection markdown files on disk. If not, it fails with `index looks empty or stale, run stardust index` rather than writing a misleading empty registry.

</details>

<details>
<summary><b>Consequences</b></summary>
<br>

- Renames self-heal on the next incremental index pass.
- `docs/INDEX.md` is not rewritten to a false empty state over a populated vault.
- Registry generation may fail until the user indexes. The failure is intentional and actionable.

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Let registry read disk directly. Rejected because collection record listing, frontmatter decoding, and sort behavior already live behind the indexed records API.
- Auto-run indexing inside registry. Rejected because registry should render from validated state or fail with a clear hint.
- Require full rebuild for renames. Rejected because incremental indexing is the normal maintenance path.

</details>

<details>
<summary><b>References</b></summary>
<br>

- `docs/specs/2026-06-26-1849-stardust-hardening.md`
- `internal/service/index.go`
- `internal/service/registry.go`

</details>
