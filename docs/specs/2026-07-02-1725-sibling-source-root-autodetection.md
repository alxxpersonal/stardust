---
title: Sibling source-root autodetection for GitHub wiki workspaces
status: Implemented
version: 1
date: 2026-07-02
related:
  - docs/adr/0040-sibling-source-root-autodetection.md
  - docs/plans/2026-07-02-1725-sibling-source-root-autodetection.md
  - docs/research/2026-06-27-1721-github-wiki-compatibility.md
  - internal/convention/detect.go
  - internal/convention/check.go
  - internal/config/config.go
  - internal/service/governs.go
  - internal/service/check.go
  - internal/service/status_report.go
---

# Sibling source-root autodetection for GitHub wiki workspaces

When a workspace is a GitHub wiki clone (`<name>.wiki`) and `source_root` is unset, resolve the sibling source checkout at `../<name>` automatically, but only when it is a real checkout of the same GitHub repository. Explicit `source_root` always wins; ambiguity or any doubt resolves to nothing, preserving today's behavior.

<details>
<summary><b>Problem</b></summary>
<br>

Cross-repo wiki-to-code drift already works (commit `3ae7e54`, ADR-era `source_root`): a `.wiki` clone points at a separate source repository through `source_root` in `.stardust/config.toml`, and Stardust counts source-repo commits after a wiki page's last-commit time. But it requires the user to hand-write `source_root`. The GitHub wiki compatibility research (`docs/research/2026-06-27-1721-github-wiki-compatibility.md`, prioritized improvement 8, and the closing "Left as proposals") explicitly defers the ergonomic gap: "Auto-detecting a sibling source checkout for `.wiki.git` clones."

The common on-disk layout is fixed by GitHub's own clone URLs:

```
~/code/myrepo/          source checkout, origin .../myrepo.git
~/code/myrepo.wiki/     wiki checkout,   origin .../myrepo.wiki.git
```

A wiki workspace named `myrepo.wiki` sitting next to `myrepo` has everything needed to bind drift, yet a user still has to configure the obvious. The cost of getting this wrong is asymmetric: a source root pointed at the wrong repository invents false drift findings on every page, which is worse than reporting no cross-repo drift at all.

</details>

<details>
<summary><b>Context</b></summary>
<br>

- `internal/convention/detect.go` classifies a directory with `DetectKind`. `KindGitHubWiki` is returned when `hasGitHubWikiSignal(dir)` is true: either the directory basename carries the `.wiki` suffix (`hasWikiSuffix(filepath.Base(dir))`), or the git remote `url` in the repo config ends in `.wiki`. A structural fallback (`isFlatGitHubWiki`: `Home.md` plus `_Sidebar.md` / `_Footer.md` plus hyphenated pages) also yields `KindGitHubWiki`. `hasWikiSuffix` lowercases, trims a trailing slash, trims a trailing `.git`, and tests the `.wiki` suffix. `gitConfigPath(dir)` returns the config path for a `.git` directory or a `gitdir:` gitfile, or empty when neither exists.
- `internal/config/config.go` holds `SourceRoot string` (`toml:"source_root"`, empty by default) and `ResolveSourceRoot(vaultRoot)`, which returns an absolute path for a set value (resolving a relative value against the vault root) and an empty string for an unset value.
- Three call sites read the resolved source root, all through `ResolveSourceRoot`:
  - `internal/convention/check.go` (`CheckDocs`, line 43): `governs:` patterns absent locally are looked up under `sourceRoot` (`sourceMatchesGoverns`).
  - `internal/service/governs.go` (`matchGovernedDriftRefs`, line 315): a `governs:` pattern with no local match falls back to `source_root`, producing `driftRef{source: source_repo}` bindings measured with `gitx.CommitCountSinceUnix`.
  - `internal/service/check.go` (`sourceDriftIssues`, line 162): drives `DriftDocs` and emits `source_repo` bindings as `drift` warnings.
- `internal/service/status_report.go` builds `VaultStatus` (root, initialized, kind, repository, collections, index health). `internal/cli/status.go` renders it human-readable and as JSON. There is no field for the bound source root today, so a detected binding would be invisible.
- Precedent for conservative gap-filling exists in `DetectKind` itself: the `.git` rule sits below markdown-dominance so a git-backed vault is not misclassified. This spec extends that instinct, it does not fork it.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. When `source_root` is unset and the workspace is a `.wiki`-named GitHub wiki, bind the sibling `../<name>` source checkout automatically for wiki-to-code drift.
2. Explicit `source_root` always wins: a set value is used verbatim, autodetection never runs and never overrides it.
3. Detect only a positively confirmed match: the sibling exists, is a directory, is a git checkout, and is the same GitHub repository as the wiki (remote-URL identity). Anything short of that binds nothing.
4. Route every source-root consumer through one resolver so behavior is identical across `check`, `drift`, and `governs`.
5. Make the bound source root and its origin (`configured` or `detected`) visible in `stardust status`, so a user can see what drift is bound against and why.
6. Zero behavior change when `source_root` is empty and the workspace is not a confirmable `.wiki` sibling layout: the empty-source-root path is byte-identical to today.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No autodetection for wikis identified only by git-remote URL or the flat-structural heuristic. The sibling `<name>` is derivable unambiguously only from a `<name>.wiki` directory basename; the other two detection paths leave the local sibling name a guess, so they are excluded.
- No probing of any path other than the single sibling `../<name>`. No walking parents, no scanning siblings for a matching remote, no multi-candidate resolution.
- No config schema change. `source_root` stays the one knob; autodetection only fills its gap. No `auto_source_root = false` opt-out in this pass (an explicit `source_root` already fully overrides, and the detection is conservative enough that an opt-out is unnecessary; revisit only if a false bind is ever observed).
- No change to how source-repo drift is counted or rendered once a root is bound. The `source_repo` binding, `(source repo)` labels, and `CommitCountSinceUnix` math from commit `3ae7e54` are unchanged.
- No detection when a value is already set, even if the set value is wrong or missing on disk. Explicit config is authoritative, including its mistakes.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

One rule: autodetection fills the gap, it never overrides. Explicit `source_root` is authoritative.

**Single seam.** Add `convention.ResolveSourceRoot(cfg config.Config, root string) (path string, origin string, err error)`. It lives in `internal/convention` beside `DetectKind`, `hasWikiSuffix`, and `gitConfigPath`, all of which it reuses, and `internal/service` already imports `internal/convention`. The three existing call sites stop calling `cfg.ResolveSourceRoot(root)` directly and call this resolver instead, discarding `origin` where they only need the path. `config.ResolveSourceRoot` stays as the explicit-value primitive the new resolver delegates to.

**Resolution order.**

1. `raw := strings.TrimSpace(cfg.SourceRoot)`. If non-empty, return `cfg.ResolveSourceRoot(root)` with origin `configured`. Explicit wins, no probing.
2. Otherwise attempt sibling autodetection (below). On a confirmed match, return the absolute sibling path with origin `detected`.
3. Otherwise return `"", "", nil`: no source root, origin empty, exactly today's empty behavior.

**Sibling autodetection, every condition required.**

| Step | Condition | On failure |
|---|---|---|
| 1 | `base := filepath.Base(root)` has the `.wiki` suffix (`hasWikiSuffix(base)`) | bind nothing |
| 2 | `DetectKind(root) == KindGitHubWiki` (no error) | bind nothing |
| 3 | `name := stripWikiSuffix(base)` is non-empty (`foo.wiki` -> `foo`) | bind nothing |
| 4 | `sibling := filepath.Join(filepath.Dir(root), name)` exists and is a directory | bind nothing |
| 5 | `gitConfigPath(sibling) != ""`: the sibling is a git checkout | bind nothing |
| 6 | remote-URL identity match: the wiki remote equals the sibling remote as the same repo | bind nothing |

On all six passing, the result is `filepath.Clean(sibling)`, origin `detected`. Any error, missing piece, or mismatch yields `"", ""`.

**Remote-URL identity (step 6), the locked correctness guard.** Read the first `url = ...` line from each side's git config (`gitConfigPath(root)` for the wiki, `gitConfigPath(sibling)` for the source), reusing the same scan `hasGitHubWikiSignal` already performs. Canonicalize each remote to a scheme-and-user-independent `host/owner/repo` identity: lowercase, trim a trailing slash, drop a leading scheme (`https://`, `http://`, `ssh://`, `git://`), drop any `user@` host prefix, rewrite the scp-form `host:owner/repo` colon to a slash, then strip a trailing `.git` and a trailing `.wiki`. The wiki side reduces `.../owner/repo.wiki.git` to `host/owner/repo`; the source side reduces `.../owner/repo.git` to the same. Bind only when both canonical identities are non-empty and equal. If either side has no usable remote, they are treated as not equal and nothing binds. This is the guard that makes a wrong bind (the one outcome worse than none) nearly impossible: a same-named sibling that is a different repository fails the identity check.

**Visibility.** Add `SourceBinding` to `VaultStatus`:

```go
type SourceBinding struct {
    Path   string `json:"path,omitempty"`
    Origin string `json:"origin,omitempty"` // configured | detected
}
```

`GatherStatus` computes it via `convention.ResolveSourceRoot`; an empty path leaves `Origin` empty and the human renderer omits the line. `writeStatusHuman` prints `source root: <path> (<origin>)` when set. JSON is emitted by the existing struct-tag path. This is the surface where a user sees the bound root and whether it was configured or detected. The existing `(source repo)` drift labels in `check` and `drift` output are unchanged and remain the per-binding surface.

</details>

<details>
<summary><b>Alternatives</b></summary>
<br>

- **`.git`-only acceptance, no URL match.** Simpler, but a sibling `<name>` that happens to be an unrelated git checkout would bind and manufacture false drift on every page. Rejected: the hard constraint is that a wrong bind is worse than no bind, and the URL machinery is already in `detect.go`.
- **Derive `<name>` from the wiki's git-remote URL, not the directory name.** Covers wikis cloned into an off-name directory, but the remote name need not match any local sibling directory, so it trades a predictable rule for a guess. Rejected in favor of the unambiguous basename rule; the URL is used only to confirm identity, never to name the sibling.
- **Probe several parent or sibling candidates for a matching remote.** More permissive, but multiplies filesystem and git-config reads and opens ambiguity (two matches, which wins). Rejected: one deterministic candidate keeps the rule legible and the cost bounded.
- **A `auto_source_root` config toggle.** New surface for no gain: explicit `source_root` already overrides fully, and the detection is conservative by construction. Deferred to a real observed need.
- **Fill the gap inside `config.ResolveSourceRoot`.** It is a pure config method with no knowledge of directory kind or the filesystem beyond the vault root; teaching it to probe siblings would tangle config with convention detection. A resolver in `internal/convention` keeps the layering clean.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- **False bind (the worst case).** A same-named sibling git checkout that is a different repository. Mitigation: the remote-URL identity match rejects it; only a same-repo remote passes.
- **No-remote local clones.** A `foo.wiki` and `foo` created locally without remotes cannot be identity-confirmed and bind nothing. Accepted: the escape hatch is an explicit `source_root`, and silence beats a guess here.
- **Fork or rename skew.** A wiki whose remote and the sibling's remote resolve to different `owner/repo` (a fork checked out beside an upstream wiki) fails the match and binds nothing. Correct and conservative; explicit config resolves it.
- **scp versus https remote forms.** `git@host:owner/repo.git` and `https://host/owner/repo.git` must canonicalize equal. Mitigation: the canonicalizer strips scheme and user and rewrites the scp colon before comparison; tested across both forms.
- **Cost.** The resolver runs per `check` / `drift` / `status`. It adds one `DetectKind` readdir (already paid in the drift and status paths), a couple of `os.Stat`s, and up to two small git-config reads, all local. Negligible, and it short-circuits at step 1 for any non-`.wiki` workspace.
- **Detection drift versus configured intent.** A user who later sets `source_root` silently switches origin from `detected` to `configured`. That is the intended precedence and is visible in `stardust status`.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- Unit (`internal/convention`): `ResolveSourceRoot` returns `configured` for a set `source_root` and never probes; returns `detected` for a `<name>.wiki` workspace with a same-repo sibling checkout; returns empty for each single missing condition (not `.wiki`-named, not `KindGitHubWiki`, sibling missing, sibling is a file, sibling has no `.git`, remote mismatch, remote absent on either side).
- Unit: the URL canonicalizer maps `https://host/owner/repo.wiki.git`, `git@host:owner/repo.wiki.git`, and `ssh://git@host/owner/repo.wiki.git` to one identity, and matches the source forms without the `.wiki`.
- Unit: explicit `source_root` set to a wrong or missing path still resolves as `configured` and suppresses autodetection (explicit wins, including its mistakes).
- Integration: seed a `foo.wiki` workspace and a sibling `foo` source repo with matching remotes; assert cross-repo drift binds with no `source_root` set, identical to the explicit-`source_root` result; assert an unrelated-remote sibling binds nothing.
- Status: `stardust status` shows `source root: <path> (detected)` for the sibling case and `(configured)` for the explicit case; omits the line when nothing is bound; JSON carries `source.path` and `source.origin`.
- Regression: the three existing `source_root` tests (`TestDriftDocsUsesSourceRootForWikiGoverns`, `TestDriftDocsSourceRootCleanWhenSourceUnmoved`, `TestDriftDocsEmptySourceRootKeepsSameRepoResolution`) stay green unchanged.
- Gate: `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` empty, zero U+2014 / U+2013, `stardust check` exit 0.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. `internal/convention`: add `ResolveSourceRoot(cfg, root)` and the `stripWikiSuffix` / `remoteURL` / canonicalize helpers; unit-test every branch.
2. Route `internal/convention/check.go`, `internal/service/governs.go`, and `internal/service/check.go` through the new resolver, discarding `origin` where unused.
3. `internal/service/status_report.go`: add `SourceBinding`, populate it in `GatherStatus`; `internal/cli/status.go`: render the `source root` line; test both output modes.
4. Integration coverage: sibling match, unrelated-remote sibling, no-remote sibling, explicit override; confirm the three existing drift tests are unchanged.
5. Mark research improvement 8 and the "Left as proposals" item shipped; regenerate the docs index.

</details>
