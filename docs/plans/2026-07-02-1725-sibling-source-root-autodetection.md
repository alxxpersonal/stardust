---
title: Sibling source-root autodetection - implementation plan
status: Done
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-1725-sibling-source-root-autodetection.md
  - docs/adr/0040-sibling-source-root-autodetection.md
  - internal/convention/detect.go
  - internal/convention/check.go
  - internal/config/config.go
  - internal/service/governs.go
  - internal/service/check.go
  - internal/service/status_report.go
---

# Sibling source-root autodetection - implementation plan

Add a single `convention.ResolveSourceRoot` seam that keeps explicit `source_root` authoritative and, only for a `<name>.wiki` GitHub wiki workspace, binds the sibling `../<name>` source checkout when a remote-URL identity match confirms it is the same GitHub repository. Route the three drift call sites through it and surface the binding in `stardust status`.

## Header

- **Goal:** cross-repo wiki-to-code drift with zero config when a `<name>.wiki` workspace sits next to a same-repo `<name>` checkout; explicit `source_root` always wins; a wrong bind is impossible by construction.
- **Architecture:** one resolver in `internal/convention` beside `DetectKind`, reused by `check`, `drift`, and `governs`; `config.ResolveSourceRoot` stays the explicit-value primitive; a `SourceBinding` on `VaultStatus` makes the result visible.
- **Tech stack:** Go 1.26, `os` / `path/filepath` / `bufio` (all already imported in `detect.go`). No new dependencies.
- **Global constraints:** conventional commits, imperative lowercase, no co-author or generated-by trailers, `go build ./...` + `go test ./...` + `make lint` green, `gofmt -l .` empty, `stardust check` exit 0, ZERO U+2014 / U+2013.

## Context

Cross-repo drift shipped in commit `3ae7e54` behind an explicit `source_root`, no auto-detect. This plan implements spec `2026-07-02-1725-sibling-source-root-autodetection.md` and ADR 0040: fill the gap only when it can be positively confirmed. `internal/convention/detect.go` already owns the `.wiki` suffix test (`hasWikiSuffix`), the `.git` locator (`gitConfigPath`), and git-config `url` scanning (`hasGitHubWikiSignal`), so the resolver reuses that machinery and adds no dependency.

## Reuse map (read first)

- `internal/convention/detect.go` - `DetectKind`, `KindGitHubWiki`, `hasWikiSuffix`, `gitConfigPath`, `hasGitHubWikiSignal` (the git-config `url` scan to mirror for `remoteURL`).
- `internal/config/config.go` - `Config.SourceRoot`, `Config.ResolveSourceRoot` (the explicit-value primitive to delegate to).
- `internal/convention/check.go` - `CheckDocs` line 43, the first call site.
- `internal/service/governs.go` - `matchGovernedDriftRefs` line 315, the second call site.
- `internal/service/check.go` - `sourceDriftIssues` line 162, the third call site.
- `internal/service/status_report.go` - `VaultStatus`, `GatherStatus`; `internal/cli/status.go` - `writeStatusHuman` for the human render.

## Task 1: build the resolver, wiring, and visibility

- Modify: `internal/convention/detect.go` (or a new `internal/convention/source_root.go`), `internal/convention/check.go`, `internal/service/governs.go`, `internal/service/check.go`, `internal/service/status_report.go`, `internal/cli/status.go`
- Test: `internal/convention/*_test.go`, `internal/service/drift_test.go`, `internal/service/status_report_test.go`, `internal/cli/status_test.go`

- [x] Add `convention.ResolveSourceRoot(cfg config.Config, root string) (path, origin string, err error)`: trimmed non-empty `SourceRoot` returns `cfg.ResolveSourceRoot(root)` with origin `configured`; empty attempts sibling autodetection; a confirmed match returns `filepath.Clean(sibling)` with origin `detected`; otherwise `"", "", nil`.
- [x] Add `stripWikiSuffix(base string) string` mirroring `hasWikiSuffix`'s trimming (`foo.wiki` -> `foo`, guard empty).
- [x] Add `remoteURL(dir string) string`: read `gitConfigPath(dir)`, scan for the first `url = ...` line (mirror the loop in `hasGitHubWikiSignal`), return the trimmed value or empty.
- [x] Add the URL canonicalizer: lowercase, trim trailing slash, drop scheme (`https://`, `http://`, `ssh://`, `git://`), drop `user@`, rewrite the scp `:` to `/`, strip trailing `.git` then `.wiki`; return `host/owner/repo` or empty. Add `sameRepoIdentity(wikiURL, srcURL) bool` requiring both non-empty and equal.
- [x] Implement the six-condition gate: `.wiki` basename, `KindGitHubWiki`, non-empty stripped name, sibling exists and is a directory, `gitConfigPath(sibling) != ""`, `sameRepoIdentity(remoteURL(root), remoteURL(sibling))`.
- [x] Unit-test every branch: configured (no probe), detected (full match), and each single-condition failure (not `.wiki`-named, not a wiki, sibling missing, sibling is a file, no `.git`, remote mismatch, remote absent either side); canonicalizer across https / scp / ssh forms; explicit wrong or missing `source_root` still resolves `configured`. Run, loop to green.
- [x] Commit `feat(convention): resolve wiki source root from a confirmed sibling checkout`.
- [x] Route the three call sites through `convention.ResolveSourceRoot`, discarding `origin` where unused: `CheckDocs`, `matchGovernedDriftRefs`, `sourceDriftIssues`. Keep `config.ResolveSourceRoot` as the delegated primitive.
- [x] Confirm `TestDriftDocsUsesSourceRootForWikiGoverns`, `TestDriftDocsSourceRootCleanWhenSourceUnmoved`, and `TestDriftDocsEmptySourceRootKeepsSameRepoResolution` pass unchanged; add an integration test binding a `foo.wiki` workspace to a same-remote sibling `foo` with no `source_root`, and one asserting an unrelated-remote sibling binds nothing. Run, loop to green.
- [x] Commit `feat(service): bind wiki drift through the shared source-root resolver`.
- [x] Add `SourceBinding{Path, Origin}` to `VaultStatus`; populate it in `GatherStatus` via the resolver; render `source root: <path> (<origin>)` in `writeStatusHuman`, omitting the line when the path is empty; confirm JSON carries `source.path` / `source.origin`.
- [x] Test status in both output modes for detected, configured, and none. Run, loop to green.
- [x] `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` green.
- [x] Commit `feat(status): surface the bound source root and its origin`.

## Task 2: review, document, and gate

- Modify: `docs/research/2026-06-27-1721-github-wiki-compatibility.md`, this plan, the spec
- Verify: full gate and self-review

- [x] Self-review against the ADR: explicit `source_root` is never overridden; the sibling is basename-derived only; all six conditions are required; the remote-URL match is mandatory, not advisory; the resolver short-circuits at the basename check.
- [x] Mark research improvement 8 and the "Left as proposals" sibling-autodetect line shipped, referencing this spec and ADR 0040.
- [x] Set the spec `status` to `Implemented` and this plan `status` to `Done`.
- [x] Regenerate the docs index: `stardust index && stardust registry`.
- [x] Full gate: `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` empty, zero U+2014 / U+2013 in touched files, `stardust check` exit 0.
- [x] Commit `docs(convention): mark sibling source-root autodetection shipped`.

## Verification

- Resolver: `configured` never probes; `detected` requires all six conditions; every single failure binds nothing; the canonicalizer unifies https / scp / ssh and matches source to wiki minus `.wiki`.
- Call sites: `check`, `drift`, and `governs` bind identically through the one resolver; the three existing `source_root` tests are unchanged.
- Integration: a same-remote sibling binds with no config, matching the explicit-`source_root` result; an unrelated-remote sibling and a no-remote sibling bind nothing.
- Status: `stardust status` shows the path and `configured` or `detected`, omits the line when nothing binds, and JSON carries both fields.
- `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` green; `stardust check` exit 0; zero em or en dashes.

## Self-review gate

- Every spec Work-breakdown item maps to a step here.
- Explicit `source_root` wins in every path; autodetection only fills an empty value.
- The remote-URL identity match is required; a same-named different-repo sibling cannot bind.
- The sibling is derived only from a `<name>.wiki` basename; URL-only and structural wikis are excluded.
- Source-repo drift counting and `(source repo)` labels from commit 3ae7e54 are unchanged.
