---
title: Stardust v0.5.0 public release
status: Draft
version: 1
date: 2026-06-27
related: [docs/research/2026-06-27-1721-github-wiki-compatibility.md, .github/workflows/ci.yml, README.md]
---

Thesis: Ship stardust as a public, brew-installable v0.5.0 with full GitHub wiki mode and a self-indexing, zero-warning docs vault, executed autonomously with Codex on the heavy builds and a human-gated final release flip.

<details>
<summary><b>Problem</b></summary>
<br>

Stardust is feature-complete for a first public cut but is not shippable: the repo is private and 25 commits ahead of an unpushed origin, there is no binary distribution (no brew, no GoReleaser, the binary reports version `unknown`), GitHub wiki support stops at the quick-wins (relative markdown links and wiki-to-code coherence are unbuilt), the README is internal-facing, and `stardust check` on its own repo still emits warnings, so the tool does not cleanly dogfood its own docs.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

- Stardust is a public GitHub repository with all session work pushed to `origin/main`.
- `brew install alxxpersonal/tap/stardust` installs a versioned binary that reports `v0.5.0`.
- GitHub wiki mode is complete: markdown relative links are graph edges, a wiki is auto-detected, and a wiki page can bind to code for drift.
- `stardust check` on the stardust repo is zero errors and zero warnings; the repo is a registered self-indexing vault.
- CI is green on `main`, and `v0.5.0` is tagged and released with cross-platform binaries.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- The Obsidian plugin release, the MCP/HTTP API surface, and homebrew-core submission (own tap only for v0.5.0).
- Non-markdown wiki formats (AsciiDoc, Textile).
- Rotating or changing any credentials beyond confirming none are committed.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

Nine phases, executed in order, Codex for the heavy Go builds:

1. Plugin updatable: commit the slash commands, bump plugin to 0.5.0, advertise `/stardust:execute`. (Done.)
2. Wiki mode (Codex): markdown relative links as graph edges, `github-wiki` detection in `internal/convention/detect.go`, subdirectory-relative resolution, and an optional wiki-to-code binding for drift. From the wiki-compatibility research doc.
3. Self-indexing vault: register the stardust repo as a docs-convention vault, fix every `stardust check` warning (missing-title on plugin command and structural files, orphan docs) to reach 0/0.
4. README for public: rewrite for a new reader (what, why, install, quickstart, the TUI, the agent workflow, the plugin), keep the badges, add the brew install line.
5. Brew release CI/CD: add `.goreleaser.yaml` (darwin/linux x amd64/arm64), a release workflow on tag, version ldflags in `main.go` so `--version` reports the tag, and create the `homebrew-tap` repo.
6. CI green: push `main` to origin, confirm the existing CI (build/vet/test/lint/plugin) passes.
7. Secret sweep: a final authoritative scan of tree and history before the visibility flip.
8. Make public: flip the repo to public via `gh`.
9. Release: tag `v0.5.0`, push the tag, GoReleaser cuts the release and updates the tap.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- Secret exposure on the public flip. Mitigation: an authoritative tree+history secret sweep gates phase 8; verified clean of real `sk-`/`ghp_`/`Bot` tokens and of committed auth files so far.
- The brew tap push needs a cross-repo token. The default Actions `GITHUB_TOKEN` cannot push to a separate tap repo, so a `HOMEBREW_TAP_TOKEN` secret is required; this is the one step that may need a human-provided PAT.
- Wiki-to-code coherence crosses repo boundaries (a `.wiki.git` is separate from source), so the binding design must be explicit and may ship as opt-in only.
- Making the repo public and cutting a release are outward-facing and hard to reverse; both are gated on the secret sweep and CI green.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- `go build ./... && go test ./... -race && golangci-lint run` green; CI green on `origin/main`.
- `stardust check` on the stardust repo prints 0 errors, 0 warnings.
- A scratch GitHub-wiki-shaped dir indexes, the graph resolves `[[Page Name]]` and `[label](Page-Name)` links, and broken-link detection is correct.
- `gh repo view` shows public; the `v0.5.0` release lists darwin/linux binaries; `brew install alxxpersonal/tap/stardust` then `stardust --version` prints `v0.5.0`.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

The executable plan at docs/plans/2026-06-27-1813-stardust-v0-5-0-public-release.md carries the per-phase tasks. Heavy Go phases (2, 5) go to Codex; git, release, and visibility phases (1, 6, 7, 8, 9) and the docs phases (3, 4) are driven here with Codex assist.

</details>

<details>
<summary><b>References</b></summary>
<br>

- docs/research/2026-06-27-1721-github-wiki-compatibility.md (wiki mode scope and proposals)
- `.github/workflows/ci.yml` (the existing Go + plugin CI)
- GoReleaser homebrew tap docs; GitHub wiki and Gollum link docs (cited in the research doc)

</details>
