---
title: Stardust hooks compose, never clobber
status: Draft
version: 1
date: 2026-06-25
related:
  - internal/hooks/hooks.go
  - internal/cli/hooks.go
  - internal/cli/init.go
  - docs/adr/0007-stardust-composes-hooks-never-clobbers.md
  - docs/adr/0008-sentinel-block-hook-injection.md
---

# Stardust hooks compose, never clobber

When a repo already has a hooks manager or its own hooks, stardust appends its index and registry calls into the existing chain instead of seizing `core.hooksPath`.

<details>
<summary><b>Problem</b></summary>
<br>

`hooks.Install` (`internal/hooks/hooks.go:72`) ends with `git config core.hooksPath .stardust/hooks`. That single line seizes the repo's hooks path. Any pre-existing hook chain stops running:

- husky (which sets `core.hooksPath` to `.husky`) is silently bypassed.
- a hand-written `.git/hooks/post-commit` (the git default path) is bypassed.
- any other manager that set a non-default `core.hooksPath` is overwritten.

This is not theoretical. exo-jobs has a git-cliff pre-commit hook; running `stardust init --docs` there pointed `core.hooksPath` at `.stardust/hooks`, which can sideline that hook. A docs tool must not quietly disable a repo's existing commit automation.
</details>

<details>
<summary><b>Context and background</b></summary>
<br>

- Current install (`internal/hooks/hooks.go`): writes `post-commit`, `post-merge`, `post-rewrite`, and an optional `pre-commit` (warn or strict) into `.stardust/hooks`, then sets `core.hooksPath` to that dir. Hooks are async and never fail a commit.
- The hook bodies are guarded (`command -v stardust >/dev/null 2>&1 && ... || true`), so they are already safe to drop into any shell hook.
- `Uninstall` does `git config --unset core.hooksPath`, which is wrong when stardust never set it (it would disable a manager stardust did not install).
- git resolves exactly one `core.hooksPath`. There is no native chaining; composition means writing into the file the active path already uses.
</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. `stardust hooks install` (and the `init` hook wiring) detect an existing hooks manager or existing hooks before touching anything.
2. When one exists, stardust appends its index and registry invocations into the existing `post-commit` (and `post-merge`) hook file, marked by a sentinel block, and does NOT change `core.hooksPath`.
3. Pure repos with no manager and no existing hooks keep the current behavior: stardust owns `.stardust/hooks` via `core.hooksPath`.
4. Install is idempotent: a second run replaces the sentinel block, never duplicates it.
5. `hooks uninstall` removes only stardust's contribution: it strips the sentinel block from a composed chain, and only unsets `core.hooksPath` when stardust set it.
</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No hooks-manager of stardust's own, and no replacement for husky or lefthook.
- No change to what the hooks DO (index, registry, check); only where they are installed.
- No multi-line orchestration engine. One sentinel block appended to one file per event.
- No Windows-specific hook shell handling beyond what exists.
</details>

<details>
<summary><b>Approach</b></summary>
<br>

One rule: stardust adds itself to the chain, never takes the chain over.

**Detection** (a new `hooks.detect(root) (mode, target)`):

| Condition | Mode | Target |
|---|---|---|
| `core.hooksPath` set to `.stardust/hooks` already | owned | `.stardust/hooks` (idempotent re-run) |
| `core.hooksPath` set to anything else (husky `.husky`, custom) | compose | that dir |
| `core.hooksPath` unset, but `.git/hooks/post-commit` (or post-merge) exists and is non-empty | compose | `.git/hooks` |
| `core.hooksPath` unset, no existing hooks | owned | `.stardust/hooks` (current behavior) |

**Compose mode**: for each event stardust cares about (`post-commit`, `post-merge`), open the target hook file (create it with a shebang if absent), and inject a sentinel block:

```sh
# >>> stardust >>> (managed block, do not edit)
command -v stardust >/dev/null 2>&1 && stardust index --since HEAD~1 --background >/dev/null 2>&1 || true
command -v stardust >/dev/null 2>&1 && stardust registry >/dev/null 2>&1 || true
# <<< stardust <<<
```

If the block is already present, replace it in place (idempotent). Never touch lines outside the block. `core.hooksPath` is left exactly as it was.

**Owned mode**: unchanged from today (write `.stardust/hooks`, set `core.hooksPath`).

**Uninstall**: if a sentinel block exists in any composed target, strip it. Only run `git config --unset core.hooksPath` when the current value is `.stardust/hooks` (stardust set it). Never unset a value stardust did not write.

The `pre-commit` check gate (warn/strict) follows the same rule: in compose mode it appends a guarded sentinel block to the existing `pre-commit`, in owned mode it writes `.stardust/hooks/pre-commit` as today.
</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- **Keep seizing `core.hooksPath`.** Rejected: it is the bug; it silently disables the repo's existing automation.
- **Move the existing hooks into `.stardust/hooks` and call them from there.** Rejected: relocating another tool's files is invasive and fragile across managers; appending a block to the file the manager already owns is minimal and reversible.
- **Refuse to install when a manager is present and tell the user to add the lines.** Rejected: worse ergonomics; the sentinel-block append is safe and idempotent, so stardust can do it cleanly.
</details>

<details>
<summary><b>Risks</b></summary>
<br>

- Target hook file is not a POSIX shell script (rare custom interpreter). Mitigation: the injected lines are POSIX sh and guarded; detect a non-`sh` shebang and fall back to a clear message rather than appending.
- A manager regenerates its hook files and drops the block. Mitigation: re-running `hooks install` is idempotent and re-adds it; document this.
- Concurrent edits to the hook file. Mitigation: read-modify-write the whole file once; the operation is not hot-path.
</details>

<details>
<summary><b>Open questions</b></summary>
<br>

- Should compose mode also wire `post-rewrite`, or only `post-commit` + `post-merge`? Default: match the current owned set (post-commit, post-merge, post-rewrite) for parity.
- Should `init` print which mode it chose (owned vs compose) so the user knows what happened? Default: yes, one line.
</details>

<details>
<summary><b>Verification</b></summary>
<br>

- Unit: detect returns `compose`/`.husky` when `core.hooksPath=.husky`; `compose`/`.git/hooks` when a `.git/hooks/post-commit` exists; `owned` otherwise.
- Compose append: given an existing `.husky/post-commit` with user lines, after install the user lines are intact AND the sentinel block is present. Run twice, assert exactly one block.
- Owned unchanged: a clean repo still gets `.stardust/hooks` + `core.hooksPath` set.
- Uninstall: strips the block from a composed file but leaves user lines; only unsets `core.hooksPath` when stardust owned it.
- Integration: in a temp repo with husky, `stardust hooks install` then a commit fires BOTH husky's hook and stardust's index.
- `go test ./...` green, `gofmt` clean, `make lint` clean, no em or en dashes.
</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- Migrating exo-jobs off its current owned-mode install (a follow-up once this ships).
- Any change to the index, registry, or check behavior.
</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. `hooks.detect(root) (mode, targetDir)`.
2. Sentinel-block read-modify-write helpers (inject, replace, strip).
3. `Install` branches on detect: compose appends blocks, owned keeps current path.
4. `Uninstall` strips the block and only unsets `core.hooksPath` when owned.
5. `init` + the `hooks` command print the chosen mode.
</details>

<details>
<summary><b>References</b></summary>
<br>

- `internal/hooks/hooks.go` (Install, Uninstall, the hook bodies)
- `internal/cli/hooks.go`, `internal/cli/init.go`
- git `core.hooksPath` documentation
- ADRs 0007, 0008
</details>
