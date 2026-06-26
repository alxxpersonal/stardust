---
title: Init auto-detects repo vs vault to default the docs convention
type: adr
status: Accepted
created: 2026-06-26
updated: 2026-06-26
related:
  - internal/cli/init.go
  - internal/convention/convention.go
  - docs/specs/2026-06-26-2104-init-detect-and-status.md
---

# 0027. Init auto-detects repo vs vault to default the docs convention

## Context

`stardust init` scaffolds the docs collections (specs, plans, adr, research) only when `--docs` is passed, with no detection and no way to assert the opposite. A code repo that wants the docs convention silently gets a bare vault. The directory contents already tell us the right default: a code repo wants docs, a human markdown vault does not.

## Decision

Add a pure detection function `convention.DetectKind(dir) (Kind, error)` that classifies the top level of a directory (non-recursive) with a fixed, documented precedence, first match wins:

1. an `.obsidian` directory present, to `KindPlainVault`
2. a source marker present (`go.mod`, `package.json`, `Cargo.toml`, or any `*.go`/`*.ts`/`*.py`/`*.rs` file), to `KindCodeRepo`
3. markdown-dominant (at least one `*.md` and `*.md` count greater than or equal to non-markdown regular files, dotfiles excluded), to `KindPlainVault`
4. a `.git` directory present, to `KindCodeRepo`
5. otherwise, to `KindPlainVault`

`init` gains a `--no-docs` flag. The final scaffold decision: explicit `--docs` always scaffolds; explicit `--no-docs` never scaffolds; with neither flag set, `DetectKind` decides and `init` prints one line naming what was detected and the override flag. If both flags are set, `--docs` wins. A `DetectKind` error is non-fatal and falls back to plain vault so init never fails on a directory it could merely sniff.

`.git` ranks below markdown-dominance deliberately: a Stardust vault is itself a git repo, so `.git` alone must not reclassify a human markdown vault as a code repo. It only breaks ties for non-markdown, non-obsidian, no-source-marker directories.

## Consequences

- First-run does the right thing without a flag in the common cases (repo root gets docs, obsidian or markdown vault does not), while the two flags give explicit override and the printed line always names the escape hatch.
- `--docs` stays backward compatible; existing `init --docs` callers are unaffected.
- Detection is one small pure function, table-testable in isolation, owned by `convention` alongside the docs convention it serves.
- A mixed repo (docs site that is also code) can be misclassified; the override flags and the printed detection line are the mitigation.

## Alternatives considered

- Detection in `service` or a new `detect` package. Rejected: it is a convention question, `convention` already owns the docs convention, and a new package is overhead.
- A tri-state `--docs=auto|on|off` string flag. Rejected: two bools with cobra `Changed()` is idiomatic and keeps `--docs` backward compatible.
- Treat `.git` as a primary code signal above markdown-dominance. Rejected: Stardust vaults are git-backed, so `.git` alone must not flip a markdown vault.
- Recursive content sniff. Rejected as premature; top-level markers classify the real cases and stay fast.

## References

- `docs/specs/2026-06-26-2104-init-detect-and-status.md`
- `internal/cli/init.go`, `internal/convention/convention.go`
