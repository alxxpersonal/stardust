---
title: Stardust v0.5.0 public release
status: Done
version: 1
date: 2026-06-27
related: [docs/specs/2026-06-27-1813-stardust-v0-5-0-public-release.md]
---

Goal: ship stardust public, brew-installable, v0.5.0, with full wiki mode and a zero-warning self-indexing docs vault. Architecture and constraints in the spec. Heavy Go builds go to Codex; git, release, and visibility steps are driven here. No co-author or generated-by trailers; no em or en dashes; never commit secrets.

## Phase 1: Plugin updatable (done)

- [x] Bake the spec workflow and the doc workflow into the slash commands; add execute
- [x] Bump plugin.json to 0.5.0, advertise /stardust:execute
- [x] Commit and merge to main (51f9844)

## Phase 2: Wiki mode, full (Codex)

- [x] Parse markdown relative links `[label](Page-Name)`, `[x](Page-Name.md)`, `./` and `../` targets as graph edges; skip URLs, anchors, images, assets
- [x] Add `github-wiki` kind to `internal/convention/detect.go` (a `.wiki`-named repo, or all-flat hyphenated pages with no `docs/`)
- [x] Subdirectory-relative wiki resolution beyond basename
- [x] Optional wiki-to-code binding: a wiki page can declare governed code paths for drift, opt-in
- [x] Gates: build, test -race, vet, gofmt, golangci-lint, dash scan; no regression to normal repos

## Phase 3: Self-indexing vault, 0 warnings

- [x] Register stardust repo as a docs-convention vault (collections already present)
- [x] Fix missing-title warnings on plugin command files and any structural files
- [x] Resolve orphan warnings (plugin/obsidian/README.md and others)
- [x] `stardust check` prints 0 errors, 0 warnings

## Phase 4: README for public

- [x] Rewrite README for a new reader: what, why, install (brew), quickstart, TUI, agent workflow, the plugin
- [x] Keep CI and license badges; add the brew install line and a screenshot or asciicast reference

## Phase 5: Brew release CI/CD (Codex)

- [x] Add version ldflags to cmd/stardust/main.go so `--version` reports the tag
- [x] Add `.goreleaser.yaml`: builds darwin/linux x amd64/arm64, archives, checksums, the brews block pointing at the tap
- [x] Add `.github/workflows/release.yml` running GoReleaser on `v*` tag push
- [x] Create the `alxxpersonal/homebrew-tap` repo; wire the `HOMEBREW_TAP_TOKEN` secret (human-gated if a dedicated PAT is needed)

## Phase 6: CI green

- [x] Push main to origin (25+ commits)
- [x] Confirm CI (build/vet/test/lint/plugin) green on origin/main

## Phase 7: Secret sweep (gate for public)

- [x] Authoritative tree+history secret scan (gitleaks or thorough pattern scan); confirm no real tokens, keys, or auth files
- [x] Confirm `.gitignore` covers env, auth, and local state

## Phase 8: Make public

- [x] `gh repo edit alxxpersonal/stardust --visibility public --accept-visibility-change-consequences`
- [x] Confirm `gh repo view` shows public

## Phase 9: Release v0.5.0

- [x] Tag `v0.5.0`, push the tag
- [x] GoReleaser cuts the release with binaries and updates the tap
- [x] Verify `brew install alxxpersonal/tap/stardust` and `stardust --version` prints v0.5.0

## Verification

- [x] CI green on origin/main; `stardust check` 0/0; wiki end-to-end correct
- [x] Repo public; v0.5.0 release with binaries; brew install works and reports v0.5.0

## Self-review gate

- [x] No secrets committed or exposed; outward-facing steps (public, release) gated on the secret sweep and CI green
