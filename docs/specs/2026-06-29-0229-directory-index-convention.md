---
title: Managed per-directory indexes
status: Implemented
version: 1
date: 2026-06-29
related:
  - docs/adr/0037-directory-index-convention.md
  - docs/plans/2026-06-29-0229-directory-index-convention.md
  - internal/config/config.go
  - internal/service/directory_indexes.go
  - internal/service/check.go
  - internal/service/registry.go
  - internal/cli/indexes.go
---

Stardust should let a vault opt into local `INDEX.md` files for selected directory trees. Those files are human-readable navigation aids, generated from the files that already exist on disk, and they must not create duplicate-name or orphan noise in normal `stardust check` output.

<details>
<summary><b>Problem</b></summary>
<br>

Some vaults use both Stardust's global docs registry and local per-folder indexes. The UpWork vault is the concrete driver: every numbered business bucket and nested subfolder needs a stable `INDEX.md` that lists the artifacts in that directory, while `10-Code/` and client worktrees are intentionally excluded.

Before this change, those `INDEX.md` files looked like ordinary notes. Multiple folder indexes shared the same normalized name, so `stardust check` produced duplicate-name warnings. Missing local indexes were invisible to Stardust, and manually maintained indexes could drift from disk.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

- Add an opt-in config block under `[conventions.directory_indexes]`.
- Generate one managed block per configured directory.
- Preserve human-authored text outside the managed block.
- Check missing or stale directory indexes without writing.
- Suppress duplicate-name and orphan warnings for configured directory index files only.
- Sync directory indexes from the service layer and expose a CLI command.
- Keep the existing docs registry as the repo-level index.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No automatic creation of project content beyond the index files.
- No recursive content summarization with an LLM.
- No migration that enables this convention by default.
- No change to the global `docs/INDEX.md` registry format.
- No broad suppression of orphan warnings for arbitrary ignored content.

</details>

<details>
<summary><b>Configuration</b></summary>
<br>

The feature is disabled unless configured:

```toml
[conventions.directory_indexes]
enabled = true
filename = "INDEX.md"
roots = ["20-Profile", "30-Legal", "40-Finance", "50-Knowledge", "60-Clients", "90-Archive"]
ignore = ["10-Code/Worktrees", ".agents", ".claude"]
mode = "managed-block"
```

`filename` defaults to `INDEX.md`. `mode` defaults to `managed-block`; it is included so later modes can be added without changing the config shape. `roots` are vault-relative directories. Missing roots are skipped. `ignore` entries can be names or path prefixes.

</details>

<details>
<summary><b>Behavior</b></summary>
<br>

For every configured root and every non-ignored subdirectory below it, Stardust writes or checks `<dir>/<filename>`.

Each generated file has a managed block:

```md
<!-- stardust-directory-index:start -->
| Date | File | Purpose |
|---|---|---|
| 2026-06-29 | [2026-06-29-profile.md](2026-06-29-profile.md) | Profile Copy. |
<!-- stardust-directory-index:end -->
```

When an index file does not exist, Stardust creates a default heading and the managed block. When it already exists, Stardust replaces only the block and keeps any preamble or notes around it.

Entries are immediate children of that directory:

- Markdown files use their frontmatter title or H1 as purpose.
- Directories link to the child directory path. Stardust still maintains the child `INDEX.md`, but the directory link avoids turning generated navigation into graph edges.
- Other files are listed as files with a generic purpose.
- Date is derived from a leading `YYYY-MM-DD` filename prefix, else `-`.
- Entries sort newest dated files first, then undated entries by name.

</details>

<details>
<summary><b>CLI and service API</b></summary>
<br>

The service exposes:

- `SyncDirectoryIndexes(ctx)` writes indexes and returns files touched.
- `CheckDirectoryIndexes(ctx)` reports missing or stale indexes without writing.

The CLI exposes:

```sh
stardust indexes
stardust indexes --check
stardust indexes --output json
```

`stardust registry` and `RegenerateRegistry` sync directory indexes after the docs registry and manifest are refreshed, but only when directory indexes are enabled.

</details>

<details>
<summary><b>Check integration</b></summary>
<br>

`stardust check` integrates the convention in two ways:

- Configured directory index files are excluded from orphan warnings.
- Duplicate-name warnings are skipped when every duplicate path is a configured directory index.
- Missing or stale managed blocks are reported as `directory-index-missing` or `directory-index-stale` warnings.

The suppression is intentionally narrow. A document that is not a configured directory index can still be an orphan, even if it lives in a directory ignored by the directory-index convention.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- Unit tests cover config loading, syncing, preamble preservation, check drift, CLI sync, CLI check, and registry-triggered sync.
- `go test ./...`
- `go build ./cmd/stardust`
- `go run ./cmd/stardust index`
- `go run ./cmd/stardust registry`
- `go run ./cmd/stardust check --strict` remains blocked by pre-existing Stardust vault debt unrelated to this feature.

</details>
