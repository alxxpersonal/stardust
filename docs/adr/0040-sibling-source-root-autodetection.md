---
title: Wiki source root autodetects a same-repo sibling checkout only when confirmed
status: Accepted
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-1725-sibling-source-root-autodetection.md
  - docs/plans/2026-07-02-1725-sibling-source-root-autodetection.md
  - docs/research/2026-06-27-1721-github-wiki-compatibility.md
  - internal/convention/detect.go
  - internal/convention/check.go
  - internal/config/config.go
  - internal/service/governs.go
---

# Wiki source root autodetects a same-repo sibling checkout only when confirmed

When a `.wiki`-named GitHub wiki workspace has no `source_root`, Stardust binds the sibling `../<name>` for cross-repo drift only when that sibling is a git checkout of the same GitHub repository. Explicit `source_root` always wins; anything short of a remote-confirmed match binds nothing and keeps today's behavior.

## Context

Cross-repo wiki-to-code drift shipped in commit `3ae7e54` behind an explicit `source_root` config, deliberately "no auto-detect." The GitHub wiki compatibility research (`docs/research/2026-06-27-1721-github-wiki-compatibility.md`) left "Auto-detecting a sibling source checkout for `.wiki.git` clones" as an open proposal. The ergonomic gap is real: GitHub clone URLs fix the on-disk layout, so a `myrepo.wiki` workspace almost always sits next to a `myrepo` source checkout, and forcing the user to configure the obvious is friction.

The governing constraint is asymmetric cost. `source_root` feeds the drift engine: an unmatched `governs:` pattern is resolved under the source root, and commits there after a wiki page's last-commit time become drift warnings. A source root pointed at the wrong repository does not fail loudly, it invents false drift on every page. A wrong bind is strictly worse than no bind. Any autodetection must therefore be conservative to the point of refusing whenever it cannot positively confirm the target.

Three facts shape the mechanism. First, `internal/convention/detect.go` already reaches `KindGitHubWiki` three ways: a `.wiki` directory basename, a `.wiki` git-remote URL, or the flat-structural heuristic. Only the basename yields the sibling `<name>` without guessing. Second, `detect.go` already reads git-config `url` lines (`hasGitHubWikiSignal`) and resolves `.git` dir-or-gitfile locations (`gitConfigPath`), so remote-identity confirmation reuses existing machinery. Third, `config.ResolveSourceRoot` is a pure config method with no knowledge of directory kind or the filesystem, and three call sites (`convention/check.go`, `service/governs.go`, `service/check.go`) call it directly.

## Decision

Add sibling autodetection as a gap-filler behind one new resolver, and gate acceptance on a remote-URL identity match.

- **One resolver, one precedence.** Add `convention.ResolveSourceRoot(cfg, root) (path, origin, err)` and route all three call sites through it. A set `source_root` returns `config.ResolveSourceRoot`'s value with origin `configured` and never probes. An unset value attempts sibling autodetection, returning origin `detected` on success or an empty path (today's behavior) otherwise. Explicit config is authoritative, including when it is wrong or missing on disk.
- **Basename-only sibling.** The sibling name is derived solely from a `<name>.wiki` directory basename (`stripWikiSuffix`), so `<name>` is never a guess. Wikis identified only by remote URL or the structural heuristic are excluded, because their local sibling name is not determinable.
- **Single candidate, all conditions required.** Probe exactly `../<name>`. Accept only when: the basename has the `.wiki` suffix, `DetectKind` is `KindGitHubWiki`, the stripped name is non-empty, the sibling exists and is a directory, the sibling has a `.git` (`gitConfigPath != ""`), and the remote-URL identity matches. Any failure binds nothing.
- **Remote-URL identity is required, not advisory.** Reduce each side's first git-config `url` to a scheme-and-user-independent `host/owner/repo` identity (drop scheme, drop `user@`, rewrite the scp `:` to `/`, strip trailing `.git` and `.wiki`) and bind only when both are non-empty and equal. A missing remote on either side counts as no match. This single guard is what keeps a wrong bind from happening.
- **Detection is visible.** `VaultStatus` gains a `SourceBinding{Path, Origin}` populated by the resolver and rendered by `stardust status` as `source root: <path> (configured|detected)`, so the bound root and its provenance are inspectable. It is omitted when nothing binds.

## Consequences

- A `myrepo.wiki` next to a same-remote `myrepo` gets cross-repo drift with zero config, closing the research proposal and improvement 8.
- The three drift call sites share one resolver, so `check`, `drift`, and `governs` bind identically. `config.ResolveSourceRoot` remains the explicit-value primitive the new resolver delegates to.
- A same-named sibling that is a different repository, a fork beside an upstream wiki, or a no-remote local clone binds nothing. These users set `source_root` explicitly, which the resolver honors first. Silence is the deliberate outcome when identity cannot be confirmed.
- `stardust status` now answers "what is drift bound against, and did I configure it or did Stardust." Origin flips from `detected` to `configured` the moment a user sets `source_root`, which is the intended precedence made visible.
- The empty-source-root, non-`.wiki` path is byte-identical to today. The resolver short-circuits at the basename check for any non-`.wiki` workspace, so the added cost is one string test in the overwhelming common case.

## Alternatives considered

- **Accept a sibling on `.git` presence alone.** Rejected. A same-named but unrelated checkout would bind and manufacture false drift, violating the one-worse-than-none constraint. The URL machinery already exists, so the guard is cheap.
- **Name the sibling from the wiki's remote URL.** Rejected. The remote name need not match any local directory; using it to name the sibling trades a predictable rule for a guess. The URL confirms identity, it does not name the target.
- **Probe multiple parent or sibling candidates.** Rejected. It multiplies filesystem and config reads and introduces multi-match ambiguity. One deterministic candidate keeps the rule legible and bounded.
- **A `auto_source_root` opt-out toggle.** Deferred. Explicit `source_root` already overrides completely and the detection is conservative by construction; a toggle is new surface for no current need. Revisit only if a false bind is ever observed.
- **Fill the gap inside `config.ResolveSourceRoot`.** Rejected. It would couple pure config to directory-kind detection and filesystem probing. The resolver belongs in `internal/convention` alongside `DetectKind`.

## References

- docs/research/2026-06-27-1721-github-wiki-compatibility.md (improvement 8, "Left as proposals")
- commit 3ae7e54 (cross-repo wiki-to-code drift via source_root)
- `internal/convention/detect.go` (KindGitHubWiki, hasWikiSuffix, gitConfigPath, hasGitHubWikiSignal)
- `internal/config/config.go` (SourceRoot, ResolveSourceRoot)
- `internal/service/governs.go`, `internal/service/check.go` (source-repo drift bindings)
