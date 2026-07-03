---
title: Configurable agent assets directory
status: Approved
version: 1
date: 2026-07-03
related:
  - docs/adr/0048-configurable-agents-dir.md
  - docs/plans/2026-07-03-1456-configurable-agents-dir.md
  - docs/adr/0047-agent-assets-docs-home.md
---

The agent-assets home stays `docs/agents/` by default, and one `agents_dir` key in `.stardust/config.toml` relocates it, with every convention seam (checks, registry, scaffold, sync sources) following the configured path.

<details>
<summary><b>Problem</b></summary>
<br>

ADR 0047 homed shared agent assets in `docs/agents/`, but the path is hardcoded at six seams: the sync source defaults (`internal/agentsync/config.go`), the stray-doc exemption (`internal/convention/check.go:110`), the orphan exemption (`internal/service/check.go:62`), the registry Agents section discovery (`internal/service/registry.go:117`), and the init scaffold (`internal/cli/init.go:150`). A repo that wants its assets at `docs/skills/`, or any other place, cannot move them: sync.toml can override the sync sources, but the convention layer keeps validating, exempting, scaffolding, and listing the hardcoded path, so a relocated home is flagged stray, loses its registry section, and never scaffolds.
</details>

<details>
<summary><b>Context and background</b></summary>
<br>

- `.stardust/config.toml` (`internal/config.Config`) is the workspace config and already carries flat path-like knobs (`SourceRoot`) with empty-means-default semantics.
- `agentsync.DefaultConfig(home, root)` builds the source list; `LoadConfig` overlays `sync.toml` on top of it. The one production caller threads through `internal/cli/sync.go:93`; `internal/service/registry.go` builds its own scoped source list for the Agents section.
- The convention checks receive the repo root and, for docs checks, the registered collection folders; they do not read `config.Config` today in every path, so the knob must be threaded, not globally read.
- ADR 0047's semantics (placement is sharing, never stray, orphan-exempt, registry-listed, init-scaffolded, compat demotions) are location-independent; only the location is fixed.
</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. A new optional `agents_dir` key in `.stardust/config.toml`; empty or absent means `docs/agents` (zero behavior change for every existing workspace).
2. All six seams follow the configured value: sync source defaults, stray-doc exemption (when the dir is under `docs/`), orphan exemption, registry Agents section, and the `init --docs` scaffold.
3. The value is a repo-relative directory; `docs/skills`, `agents-home`, or any normalized relative path works. Kind subfolders (`skills/`, `subagents/`, `rules.md`) keep their fixed names inside it.
4. `stardust check` validates the knob itself: an absolute path, a path escaping the repo (`..`), or a path colliding with a registered collection folder is a config error with a clear message.
5. `sync.toml` explicit sources still override everything, unchanged.
</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- Configurable kind-subfolder names (`skills/`, `subagents/`, `rules.md` stay fixed).
- Multiple agent-asset homes per repo; one dir per workspace.
- Moving the global-scope homes (`~/skills`, `~/agents`); this is the repo-scope knob only.
- A migration command; moving files and re-running sync is the migration.
</details>

<details>
<summary><b>Approach</b></summary>
<br>

**The knob.** `internal/config.Config` gains `AgentsDir string` with the toml tag `agents_dir` and a comment documenting the default. A single resolver owns the default and normalization:

```go
// AgentsDir returns the repo-relative agent assets home, defaulting to
// docs/agents. The value is slash-normalized with any trailing slash trimmed.
func (c Config) AgentsDir() string
```

**Threading, per seam:**

| Seam | Change |
|---|---|
| `agentsync.DefaultConfig` | gains the resolved dir (new parameter or a variant constructor; pick the smallest API break, the sole callers are cli/sync.go and service/registry.go) and builds `repo-skills`, `repo-agents`, `repo-rules` under it |
| stray-doc exemption (`convention.CheckDocs` path) | exempts the configured dir instead of the literal, only relevant when the dir sits under `docs/`; the exemption is prefix-exact like today |
| orphan exemption (`service.Check`) | uses `s.Config.AgentsDir() + "/"` as the prefix |
| registry Agents section (`service.AgentsSection`) | discovers under the configured dir |
| init scaffold (`cli/init.go`) | scaffolds the configured dir when a config already exists at init time, else the default; the rules stub text stays identical |
| knob validation (`service.Check`) | error issue `bad-agents-dir` for absolute, escaping, or collection-colliding values |

**Config plumbing.** The seams that already hold a `config.Config` (service methods, init) read it directly. `convention.CheckDocs` gains the dir as a parameter (it already takes root and ignore, so one more argument keeps it pure). No global state.

**Docs.** README's agent-assets section and the plugin setup command gain one line each documenting `agents_dir`.
</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Deriving the home from sync.toml sources instead of a config knob: rejected, sync.toml is the sync layer's override surface, while the convention seams (checks, registry, scaffold) must not depend on parsing sync.toml, and most repos never create one.
- A `[agents]` config table with per-kind paths: rejected as YAGNI; one dir with fixed kind names covers the ask (`docs/skills` style relocation) without a config schema sprawl.
- Environment variable override: rejected; the home is a repo property, not a session property.
</details>

<details>
<summary><b>Risks</b></summary>
<br>

- A dir outside `docs/` loses doc-workflow benefits silently (no stray-doc protection is needed there, but index/search still cover it since the scan is root-wide). Mitigation: this is by design and documented; check still validates the knob shape.
- Existing repos with assets at `docs/agents` and a config pointing elsewhere would orphan the old copies. Mitigation: the old dir simply becomes normal docs content again (stray-doc will flag it under `docs/`), which is the correct signal to finish the move.
- API churn on `DefaultConfig`: bounded, two production callers.
</details>

<details>
<summary><b>Verification</b></summary>
<br>

In a temp repo with `agents_dir = "docs/skills"`: init scaffolds `docs/skills/{skills,subagents,rules.md}`; a skill under it is check-clean (no stray-doc, no orphan), registry-listed, synced into all three tool dirs by the hooks; `sync --check` gates drift. The default path (no knob) behaves byte-identically to v0.6.0 across the same flow. Invalid knob values (`/abs`, `../up`, a registered collection folder) produce the `bad-agents-dir` check error. The stardust repo itself stays 0 errors 0 warnings.
</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- Global-scope relocation, per-kind renames, multi-home support.
</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. Config knob + resolver + validation issue, threaded through all six seams, TDD per seam.
2. Docs (README + setup command line) and the adversarial review, including the byte-identical default proof.
</details>

<details>
<summary><b>References</b></summary>
<br>

- internal/config/config.go, internal/agentsync/config.go, internal/convention/check.go, internal/service/check.go, internal/service/registry.go, internal/cli/init.go
- docs/adr/0047-agent-assets-docs-home.md, docs/adr/0048-configurable-agents-dir.md
</details>

- Reviewed 2026-07-03 after implementing Task 1: the config resolver, sync defaults, check exemptions, registry section, and init scaffold now realize this spec; references hold.
