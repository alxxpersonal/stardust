---
title: Hooks compose-not-clobber - implementation plan
status: Draft
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0345-hooks-compose-not-clobber.md
---

# Hooks compose-not-clobber - implementation plan

Make `hooks.Install` detect an existing hooks chain and append a sentinel block to it instead of seizing `core.hooksPath`, keeping owned-mode behavior for bare repos.

## Header

- **Goal:** stardust adds itself to an existing hook chain (husky, custom, `.git/hooks`) without taking it over; bare repos keep the current `.stardust/hooks` + `core.hooksPath` behavior.
- **Architecture:** a `detect(root)` resolver picks owned vs compose; compose does a sentinel-block read-modify-write of the target hook files; owned is unchanged. Uninstall is made surgical.
- **Tech stack:** Go 1.26, `os/exec` git, `os` file IO. No new deps.
- **Global constraints:** conventional commits, no co-author trailers, gofmt + `go test ./...` + `make lint` green, ZERO em or en dashes. Sequenced AFTER the JSON-RPC contract work (Phases F and C) lands.

## Context

`internal/hooks/hooks.go` `Install` writes hook scripts into `.stardust/hooks` and runs `git config core.hooksPath .stardust/hooks` (line 72), which clobbers any existing chain. The fix branches install on detection (spec `2026-06-25-0345-hooks-compose-not-clobber.md`, ADRs 0007 and 0008).

## Reuse map (read first)

- `internal/hooks/hooks.go` - `Install`, `Uninstall`, the `postCommit`/`postMerge`/`preCommitWarn`/`preCommitStrict` bodies (already guarded, reuse the command lines).
- `internal/cli/hooks.go` - `newHooksCmd`, the install/uninstall plumbing + the stderr message.
- `internal/cli/init.go` - the `Wired commit hooks` line.

## Task 1: detect the hooks mode

- Create: `internal/hooks/detect.go`
- Test: `internal/hooks/detect_test.go`
- Produces: `func detect(root string) (mode, targetDir string, err error)` where mode is `owned` or `compose`.

- [ ] Write a table test: `core.hooksPath=.stardust/hooks` -> owned; `=.husky` -> compose/.husky; unset + existing `.git/hooks/post-commit` -> compose/.git/hooks; unset + nothing -> owned/.stardust/hooks.
- [ ] Run it, confirm fail.
- [ ] Implement detect: read `git config --get core.hooksPath`; classify per the spec table; for the unset case, stat `.git/hooks/post-commit` and `.git/hooks/post-merge`.
- [ ] Run; loop to green.
- [ ] Commit `feat(hooks): detect an existing hook chain`.

## Task 2: sentinel-block helpers

- Create: `internal/hooks/block.go`
- Test: `internal/hooks/block_test.go`
- Produces: `injectBlock(path, lines string) error`, `stripBlock(path string) error`, with markers `# >>> stardust >>>` / `# <<< stardust <<<`.

- [ ] Test: inject into a file with user lines keeps them and adds one block; inject twice yields exactly one block; strip removes only the block.
- [ ] Run, confirm fail.
- [ ] Implement read-modify-write: create with `#!/bin/sh` if absent, append or replace the block, set 0o755. strip removes the block and collapses blank lines.
- [ ] Run; loop to green.
- [ ] Commit `feat(hooks): add idempotent sentinel-block helpers`.

## Task 3: branch Install on the mode

- Modify: `internal/hooks/hooks.go`
- Test: `internal/hooks/hooks_test.go` (extend)

- [ ] Test: compose mode appends the index/registry block to the target `post-commit` (+ post-merge) and does NOT change `core.hooksPath`; owned mode writes `.stardust/hooks` and sets `core.hooksPath` (current behavior preserved).
- [ ] Run, confirm fail.
- [ ] Refactor `Install` to call `detect`, then: owned path = current code; compose path = `injectBlock` the index/registry lines into the target post-commit and post-merge, and the check-mode lines into the target pre-commit, leaving `core.hooksPath` untouched.
- [ ] Run; loop to green. `go build ./...`, `make lint`.
- [ ] Commit `feat(hooks): compose into an existing chain instead of seizing core.hooksPath`.

## Task 4: surgical Uninstall

- Modify: `internal/hooks/hooks.go`
- Test: `internal/hooks/hooks_test.go`

- [ ] Test: uninstall on a composed file strips the block but keeps user lines, and only unsets `core.hooksPath` when its value is `.stardust/hooks`.
- [ ] Run, confirm fail.
- [ ] Implement: strip the block from any composed target; unset `core.hooksPath` only when stardust owned it.
- [ ] Run; loop to green.
- [ ] Commit `fix(hooks): uninstall strips only stardust lines`.

## Task 5: report the chosen mode

- Modify: `internal/cli/hooks.go`, `internal/cli/init.go`

- [ ] Print `installed commit hooks (owned: .stardust/hooks ...)` or `installed commit hooks (composed into .husky ...)` so the user knows which path was taken.
- [ ] `go build ./...`, `go test ./...`, `make lint` green.
- [ ] Commit `feat(hooks): report owned vs composed install mode`.

## Verification

- detect returns the right mode for owned, husky, default-hooks, and bare repos.
- Compose append in a husky repo keeps user lines and adds exactly one block across two runs; a commit fires both husky and stardust.
- Owned mode unchanged in a bare repo.
- Uninstall strips only stardust lines; `core.hooksPath` only unset when stardust set it.
- `go test ./...`, `gofmt -l .`, `make lint` green; zero em or en dashes.

## Self-review gate

- Every spec Work-breakdown item maps to a task.
- `core.hooksPath` is never written in compose mode.
- The sentinel markers match across inject and strip.
- Owned-mode behavior is byte-identical to today.
