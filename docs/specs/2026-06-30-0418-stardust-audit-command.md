---
title: Stardust audit command
status: Draft
version: 1
date: 2026-06-30
related: [plugin/claude/commands/audit.md, plugin/claude/commands/spec.md, plugin/claude/commands/doc.md, docs/specs/2026-06-26-0418-doc-code-coherence-engine.md, docs/adr/0018-drift-detection-by-commit-distance.md, docs/adr/0019-ci-baseline-ratchet.md]
---

Thesis: `/stardust:audit` harvests the deterministic Stardust checks for the docs dimensions, fans out one subagent per code and policy dimension Stardust cannot reach, verifies every candidate refute-by-default, and writes a single ranked findings report to `docs/research/` without touching the audited code.

<details>
<summary><b>Problem</b></summary>
<br>

Stardust ships deterministic machinery for the markdown dimensions of a workspace audit (`check`, `graph`, `registry`, `sync`, the manifest drift surface), but there is no command that runs them as one pass, and nothing covers the dimensions Stardust deliberately stays out of: code semantics, secrets, dependency health, test coverage, and adherence to the repo's own `CLAUDE.md` and `.claude/rules`. A user who wants a health read today either runs the CLI verbs by hand and stitches the output, or asks an agent to "look over the code", which produces an unstructured, unverified, often hallucinated list. There is no convention-correct landing zone for the result and no discipline that keeps false positives out.

</details>

<details>
<summary><b>Context and background</b></summary>
<br>

The docs convention defines four collections (`adr`, `plans`, `research`, `specs`); there is no `audits` collection. `plugin/claude/commands/doc.md` routes "capturing a point-in-time research finding or audit" to the research note type, so the convention-consistent home for an audit report is `docs/research/`.

The deterministic dimensions are already designed and shipped. `stardust check` (internal/convention/check.go) emits `drift`, `stale-governed-doc`, `stray-doc`, `bad-doc-name`, `bad-doc-status`, `bad-doc-type`, `missing-doc-field`, `bad-doc-field`, `broken-doc-ref`, `forbidden-dash`, `governs-no-match`, and `bad-target`. `stardust graph` finds orphans and dead links. `stardust registry` regenerates `docs/INDEX.md` and `.stardust/manifest.md`. `stardust sync --check` catches skill and agent routing drift. The doc-code coherence engine (docs/specs/2026-06-26-0418-doc-code-coherence-engine.md, ADRs 0015, 0017, 0018, 0019) settles how drift is computed (path-ref plus git commit-distance, never AST or embeddings) and surfaces it as a review-prompt, never a hard fail. ADR 0019 defines the adopt-green baseline ratchet that makes a gate adoptable on a dirty repo.

The reusable agent patterns already exist in-repo: spec.md and execute.md step 1 fan out background research subagents and weigh 2-3 contrasting perspectives; every authoring command ends with a self-review gate; and the librarian cron example (docs/examples/cron-jobs/librarian/prompt.md) defines a notify, question, review finding taxonomy plus a propose-not-mutate discipline (write a proposal, never silently edit a human-owned note).

</details>

<details>
<summary><b>Goals</b></summary>
<br>

- A single `/stardust:audit` command produces one point-in-time, convention-correct findings report per run.
- Deterministic dimensions are harvested by invoking the existing Stardust checks, never reimplemented.
- Code and policy dimensions, which Stardust cannot reach, are covered by a subagent fan-out, one detector per dimension.
- Every candidate finding survives an independent, refute-by-default verification before it reaches the report, so the report stays high-signal.
- The command is adoptable on an already-dirty repo: it escalates what is new since the last baseline and lists pre-existing issues as standing backlog.
- The command proposes and never mutates the audited code.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- Designing or building remediation. The audit names problems; `/stardust:spec` and `/stardust:execute` design and build fixes.
- A new `audits` Stardust collection. The report lands in the existing `research` collection.
- Re-deriving code semantics inside Stardust. Stardust stays in the markdown lane; code analysis lives in the subagents and their language-tool priors.
- Mutating, fixing, or refactoring any audited file.
- Gating commits. The audit reports; it does not block.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

A new command file `plugin/claude/commands/audit.md`, built on the same skeleton as spec.md and execute.md: frontmatter (`description`, `argument-hint`, `allowed-tools: Bash, Read, Write, Edit, Glob, Grep, Task, TodoWrite, WebSearch, WebFetch`), the identity paragraph, the verbatim resolve-root preamble, then a `# Audit workflow` body.

The dimension catalog splits into two families.

Family A (deterministic, invoke and triage): A1 docs convention hygiene; A2 doc freshness and doc-code drift; A3 link integrity and orphans; A4 registry, index, and routing drift; A5 governs-pattern rot; A6 agent and skill target validity; A7 recency and change surface. Each maps to a specific `stardust check` issue kind or CLI verb.

Family B (code and policy, fan out one detector subagent each): B1 logic correctness; B2 concurrency and state; B3 error handling and silent failure; B4 secrets and credential exposure; B5 auth, access control, and isolation; B6 injection and untrusted input; B7 unsafe edges and boundary risk; B8 dependency and supply-chain health; B9 dead and unreachable code; B10 duplication; B11 over-complexity and low altitude; B12 reuse opportunities; B13 test coverage gaps; B14 CLAUDE.md and rule adherence. B8 merges the dependency dimension that the security and quality lenses both raised. Dimensions with a language-tool prior (B8 to B13) hand the detector that tool's output to triage rather than trust.

The process is an eight-step pipeline: explore and scope; harvest Family A; detect Family B (one detector subagent per in-scope dimension, structured JSON candidates, MUST NOT mutate); verify refute-by-default (a separate verifier subagent per candidate, tasked to refute, a finding survives only if it cannot be refuted and a concrete reachable trigger is named); synthesize and rank (dedupe cross-dimension, rank by severity times reachability, baseline ratchet); write the report; regenerate the index; self-review.

Detector and verifier are separate subagent instances so the verifier is not anchored on the detector's chain of reasoning. The verifier returns `upheld`, `refuted`, or `downgraded`; refuted findings are dropped.

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- A dedicated `audits` collection mirroring the four existing ones. Rejected as added convention surface for no gain; the research collection already fits and doc.md already routes audits there.
- A single mega-agent that detects and confirms in one pass. Rejected: an agent that confirms its own findings rubber-stamps its own hallucinations. Detector and verifier separation is the whole point.
- Trust-by-default verification (confirm the finding, list caveats). Rejected: code and security findings are the highest false-positive class from LLM auditors, and a noisy report trains the user to ignore it. Refute-by-default keeps the report trustworthy.
- Reimplementing the docs checks inside the command. Rejected: `stardust check` already owns them deterministically; the command invokes and triages.
- Having the command fix what it finds. Rejected: violates the propose-not-mutate discipline and conflates audit with remediation, which is a separate user-initiated step.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- Subagent cost and latency scale with the number of in-scope dimensions times two (detect plus verify). Mitigated by scale-to-the-ask: a narrow scope fires few dimensions.
- A verifier that is too eager to refute drops real findings. Mitigated by requiring it to name why it could refute, and by keeping detector evidence (file:line plus snippet) for spot-checking.
- Language-tool priors (B8 to B13) may be absent in a given repo. Handled by degrading to a noted gap rather than a hard fail.
- Stardust absent entirely. Handled by the grep and find fallback for the cheap dimensions and skipping the registry step.

</details>

<details>
<summary><b>Open questions</b></summary>
<br>

- Whether the baseline should be the prior audit note, `.stardust/baseline.json`, or both when they disagree. The command reads the prior note and the baseline file; reconciliation when they conflict is left to the run.
- Whether refuted candidates should always be recorded in a collapsed "considered and dismissed" note. Currently optional; recording them helps the next audit skip re-flagging.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- The command file parses as a valid Claude Code command: YAML frontmatter with `description`, `argument-hint`, `allowed-tools`, then the body.
- The resolve-root preamble matches the other commands verbatim and degrades gracefully on `MODE=none`.
- A dry run on this repo: `/stardust:audit docs` harvests Family A from `stardust check`, `graph`, `sync --check`, and the registry diff, and produces a `docs/research/<ts>-audit-<slug>.md` note with a ranked findings table and a BLUF count thesis.
- A scoped code run fans out at least one Family B detector and one verifier, and only verifier-upheld findings appear in the table.
- `grep -nP '[\x{2014}\x{2013}]'` over the command file and this spec returns nothing.
- `stardust registry` regenerates `docs/INDEX.md` with the new spec listed and, after a run, the audit note listed under research.

</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- Wiring the audit into a cron or commit hook. The command is invoked on demand; automating it is a later decision.
- A machine-readable audit findings schema beyond the per-candidate JSON the detectors emit internally.
- Remediation tooling. Fixes route through `/stardust:spec` and `/stardust:execute`.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. Write `plugin/claude/commands/audit.md` with frontmatter, identity paragraph, and the verbatim resolve-root preamble.
2. Encode the Family A and Family B dimension catalog as two tables.
3. Write the eight-step process: explore and scope, harvest A, detect B, verify refute-by-default, synthesize and rank, write the report, regenerate the index, self-review.
4. Specify the report shape: `docs/research/<ts>-audit-<slug>.md`, YAML frontmatter, collapsible research sections, BLUF count thesis, the findings table columns.
5. Add the Voice and formatting, Versioning with the research closed set, Validation checklist, and Operating rules blocks, mirroring spec.md and execute.md.
6. Verify zero em or en dashes and run `stardust registry`.

</details>

<details>
<summary><b>References</b></summary>
<br>

- `plugin/claude/commands/spec.md`, `plugin/claude/commands/execute.md`, `plugin/claude/commands/doc.md` (command skeleton and voice)
- `plugin/claude/scripts/resolve-root.sh` (workspace resolution)
- `docs/specs/2026-06-26-0418-doc-code-coherence-engine.md` (drift engine, the deterministic dimensions)
- `docs/adr/0018-*` (drift by commit-distance as a review-prompt), `docs/adr/0019-*` (CI baseline ratchet)
- `docs/examples/cron-jobs/librarian/prompt.md` (notify, question, review taxonomy and propose-not-mutate)

</details>
