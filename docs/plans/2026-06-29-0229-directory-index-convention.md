---
title: Directory index convention implementation plan
status: Done
version: 1
date: 2026-06-29
related:
  - docs/specs/2026-06-29-0229-directory-index-convention.md
  - docs/adr/0037-directory-index-convention.md
---

Goal: implement the opt-in directory index convention, including config, service sync/check, CLI command, check integration, registry integration, docs, and tests.

## Tasks

- [x] Add `Conventions.DirectoryIndexes` config structs with defaults.
- [x] Implement directory discovery from configured roots and ignores.
- [x] Implement managed-block rendering and replacement.
- [x] Implement `SyncDirectoryIndexes(ctx)`.
- [x] Implement `CheckDirectoryIndexes(ctx)`.
- [x] Suppress duplicate-name and orphan warnings for configured directory index files only.
- [x] Add `stardust indexes` with `--check` and `--output`.
- [x] Sync directory indexes from registry flows when enabled.
- [x] Add service, CLI, config, and registry tests.
- [x] Write the spec and ADR.
- [x] Run focused tests, full tests, build, index, and registry generation.

## Verification

- [x] `go test ./internal/config`
- [x] `go test ./internal/service -run 'DirectoryIndex|CheckSuppressesConfiguredDirectoryIndex|CheckReportsDirectoryIndex'`
- [x] `go test ./internal/cli -run 'Indexes|Registry'`
- [x] `go test ./...`
- [x] `go build ./cmd/stardust`
- [x] `go run ./cmd/stardust index`
- [x] `go run ./cmd/stardust registry`
- [ ] `go run ./cmd/stardust check --strict`, blocked by pre-existing vault issues outside this feature.

## Notes

The sync writes deepest directories first and links child directories with trailing slash paths, so generated navigation stays stable without becoming graph-check edges.
