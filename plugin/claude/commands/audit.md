---
description: Audit the workspace for code and docs issues, then write a verified findings report.
argument-hint: "[scope or path to audit, or empty for the whole repo]"
allowed-tools: Bash, Read, Write, Edit, Glob, Grep, Task, TodoWrite, WebSearch, WebFetch
---

You are `/stardust:audit`, the workspace audit command. It harvests the deterministic Stardust checks, fans out subagents over the dimensions no tool can reach, verifies every finding adversarially, and writes one point-in-time audit report. It proposes; it never mutates the audited code. The report is a `docs/research/` note (no `audits` collection exists, and the doc convention routes a point-in-time audit to the research type). To record a single decision the audit surfaces, use `/stardust:doc` or `/stardust:adr`; to design remediation for a non-trivial finding, use `/stardust:spec` or `/stardust:execute`. Do not print a second slash command for the user to run.

First resolve the workspace: run `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and read the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace resolved and stop; in a docs-convention repo the user can run `stardust init --docs`, and for a vault point them to `/stardust:setup`. Run every `date`, `stardust`, and file operation from `${ROOT}`. Treat `$ARGUMENTS` as the audit scope (a path, a subsystem, or a lens such as "security" or "docs"); if it is empty, audit the whole workspace at default depth. An empty argument is not a blocker, it is the full-repo default. Then run the workflow below verbatim from `${ROOT}`.

# Audit workflow

Take a point-in-time read of the workspace, surface the real problems with evidence, and leave a durable report. The deterministic dimensions (docs hygiene, freshness, links, routing) are already a Stardust tool: invoke and summarize them, do not reimplement them. The dimensions that need code semantics (correctness, security, quality, policy) get a subagent fan-out, because Stardust indexes markdown, not code, by design. Every candidate finding is then verified adversarially before it reaches the report.

**This is a read-and-report command.** It writes one research note and regenerates the index. It MUST NOT edit, fix, or refactor the audited code. Findings are a proposal the user acts on, in the librarian discipline: notify, question, or review, never a silent mutation.

## When to use

- Taking stock of a codebase plus its docs before a release, a handoff, or a refactor.
- A periodic health pass: what rotted, what drifted, what is newly risky since the last audit.
- A scoped review of one lens (security, code quality, docs coherence) or one subsystem.

Do NOT use for:

- Designing or building a fix. The audit names problems; `/stardust:spec` and `/stardust:execute` design and build the remediation.
- A single known bug with a clear fix. Fix it directly.
- Recording one decision or one research finding. Use `/stardust:doc`.

## Scale to the ask

Match depth to the request in both directions. A narrow ask ("audit the auth flow", "just the docs") collapses to the 2-3 dimensions in scope and a tight file set. A breadth signal ("exhaustive", "harden", "production-grade", "every angle") fires the full dimension catalog, deeper code paths, and dependency scans. Size that comes from real coverage is correct; size that comes from padding is not. Never invent findings to fill a quota, and never truncate scope the user asked for.

## Prerequisites

- The repo follows the docs convention: a `docs/` folder with `specs/`, `plans/`, `adr/`, `research/`. The report lands in `docs/research/`. If the folder is missing, scaffold it (`stardust init --docs` when Stardust is set up, or create the folders).
- Stardust is optional but recommended. With it, the deterministic dimensions are a CLI call and the index is automated. Without it, fall back to grep and find for the cheap checks, run the subagent dimensions anyway, and skip the registry step.
- The deterministic dimensions assume Stardust owns them. Code semantics, secrets, dependency health, and policy adherence are out of Stardust's lane and are why the subagent fan-out exists.
- Follow the repo's own `CLAUDE.md` and `.claude/rules` conventions. They override these defaults where they conflict, and they are themselves an audit dimension (B14).

## Audit dimensions

Two families. Family A is deterministic: Stardust (or a grep fallback) already settles it, so the command invokes the tool and triages its output. Family B needs code semantics or policy reading, so the command fans out one detector subagent per dimension. Scope and the user's ask decide which fire.

### Family A: deterministic (invoke Stardust, do not redo)

| ID | Dimension | Source |
|----|-----------|--------|
| A1 | Docs convention hygiene: naming, required frontmatter, status enums, doc type, stray placement, forbidden dashes | `stardust check` (`bad-doc-name`, `missing-doc-field`, `bad-doc-field`, `bad-doc-status`, `bad-doc-type`, `stray-doc`, `forbidden-dash`) |
| A2 | Doc freshness and doc-code drift: docs referencing moved code by commit-distance, stale implemented specs, cross-repo drift | `stardust check` (`drift`, `stale-governed-doc`) plus `.stardust/manifest.md`; `source_root` for cross-repo |
| A3 | Link integrity and orphans: broken `related:` and wikilink targets, orphan and disconnected docs, dead links | `stardust graph` plus `stardust check` (`broken-doc-ref`) |
| A4 | Registry, index, and routing drift: stale committed `docs/INDEX.md`, skill and agent routing drift | `stardust registry` then a diff of the regenerated index; `stardust sync --check --scope all` |
| A5 | Governs-pattern rot: `governs:` globs that match nothing | `stardust check` (`governs-no-match`) |
| A6 | Agent and skill target validity: invalid agent `targets:` routing | `stardust check` (`bad-target`) |
| A7 | Recency and change surface: what changed, surfaced TODO and commitment lines | `stardust digest` |

### Family B: code and policy (fan out one detector subagent per dimension)

| ID | Dimension | What it hunts |
|----|-----------|---------------|
| B1 | Logic correctness | Off-by-one, inverted conditionals, wrong operator or variable (for example OLD vs NEW in a SQL trigger DELETE branch), type mismatches (text compared to uuid), null and zero-value mishandling, fall-through control flow, references to symbols or columns that do not exist |
| B2 | Concurrency and state | Races, TOCTOU, unguarded shared-state mutation, missing locks, non-atomic check-then-act, ordering and idempotency assumptions, concurrent-write loss |
| B3 | Error handling and silent failure | Swallowed errors, empty catch blocks, ignored return or err values, unhandled rejections or panics on a hot path, errors logged but not propagated, responses that leak internals or stack traces, fallbacks that mask a real failure |
| B4 | Secrets and credential exposure | Hardcoded keys and tokens, committed `.env` or service-account files, an admin or service-role client reachable from a client bundle, secrets in logs or error messages, real secrets in fixtures or seed data |
| B5 | Auth, access control, and isolation | Missing or bypassable auth guards, trusting untrusted request metadata for role or tenant or admin, tables without row-level security, broken object-level authorization (IDOR), privilege escalation, tenant-isolation gaps, `SECURITY DEFINER` functions that do not pin `search_path` |
| B6 | Injection and untrusted input | SQL, command, path-traversal, and template injection, SSRF, XSS or unsafe HTML render, unsafe deserialization, prototype pollution, eval of dynamic input, any trust boundary that accepts external input without validation |
| B7 | Unsafe edges and boundary risk | Destructive file operations without guards, unsafe defaults, ReDoS, missing limits or timeouts leading to resource exhaustion, panics on malformed input, unsafe shell interpolation |
| B8 | Dependency and supply-chain health | Known-vulnerable dependencies (`govulncheck` for Go, `osv-scanner`, `npm audit` or `bun audit`), unpinned or abandoned packages, lockfile drift, unused or missing module entries, license risk, dependency-confusion surface |
| B9 | Dead and unreachable code | Unreferenced exports, functions, files, and packages, unreachable branches, commented-out blocks, orphaned modules, build-tag-gated corpses (priors: `deadcode`, `staticcheck` U1000, `ts-prune`, `knip`) |
| B10 | Duplication | Near-identical blocks, repeated logic, parallel implementations that should collapse to one (priors: `dupl`, `jscpd`) |
| B11 | Over-complexity and low altitude | High cyclomatic and cognitive complexity, deep nesting, god functions, long parameter lists, abstractions built for a single caller (YAGNI) (priors: `gocyclo`, `gocognit`) |
| B12 | Reuse opportunities | Hand-rolled logic an existing in-repo utility, component, or stdlib primitive already covers; surface the existing primitive with `stardust query` and grep |
| B13 | Test coverage gaps | Untested packages and functions, uncovered critical and error paths, assertion-free tests, coverage below threshold on core or changed code (prior: the language coverage runner) |
| B14 | CLAUDE.md and rule adherence | The repo `CLAUDE.md` and `.claude/rules` parsed, then actual repo and docs state checked against each stated rule (frontmatter shape, folder layout, naming, dash and emoji bans, any never-do-X). Natural-language policy, not a schema, so it needs a reading pass |

## Process

Do not skip steps. Do not write the report before exploring and verifying.

### 1. Explore and scope first

- Get the real date and time: run `date "+%Y-%m-%d-%H%M"`. Never guess the timestamp.
- Find prior audits so this one supersedes rather than duplicates:
  - `stardust query "audit <topic>"` surfaces prior audit notes, related specs, and ADRs.
  - `stardust bundle "<scope>"` assembles task-scoped context with file paths.
- Read the prior audit note, if any, and the baseline at `.stardust/baseline.json` if present. The new audit escalates what is new since that baseline (the adopt-green ratchet), and lists pre-existing issues as standing backlog rather than re-crying every old wolf.
- Resolve scope from `$ARGUMENTS`: a path, a subsystem, or a lens narrows Family B to the dimensions and files in scope; an empty argument means the whole workspace at default depth. Detect the toolchain (Go, Node, mixed) so the dependency, complexity, and coverage priors in Family B can run.
- Scale to the ask per the section above before deciding how many dimensions fire.

### 2. Harvest the deterministic dimensions (Family A)

- Run the Family A tools from `${ROOT}` and bucket their output by dimension:
  - `stardust check --output json` for A1, A2, A3, A5, A6.
  - `stardust graph --output json` for A3 (orphans and dead links).
  - `stardust sync --check --scope all` for A4 routing drift.
  - `stardust registry`, then diff the regenerated `docs/INDEX.md` against the committed copy for A4 index drift.
  - `stardust digest` for A7.
  - Read `.stardust/manifest.md` for the free drift surface (active plans, stale implemented docs, docs referencing moved code).
- These results are already truth and need no subagent. The mechanical kinds (`forbidden-dash`, `bad-doc-name`, `bad-doc-field`, `broken-doc-ref` existence, `bad-target`) ship straight to the report as confirmed. The judgment kinds (`drift`, `stale-governed-doc`) are review-prompts by design (ADR 0018), so they enter Phase 3 verification as candidates, not facts.
- If Stardust is not available, skip this step's CLI calls, say so, and fall back to grep and find for dashes, frontmatter keys, and broken markdown links.

### 3. Detect: fan out one subagent per Family B dimension

- For each in-scope Family B dimension, dispatch one detector Task subagent on a capable model (opus 4.8 or equivalent). Hand each agent a tight scope: the relevant files and globs for its dimension (found via grep, glob, and `stardust bundle`), and for the dimensions with a deterministic prior (B8, B9, B10, B11, B13) the tool output to triage rather than trust verbatim.
- Each detector MUST read the real code path, not the docs alone, and MUST NOT write to disk or mutate anything. It only reports.
- Each detector returns structured candidate findings, not prose. One record per candidate:

```json
{
  "id": "<dimension>-<n>",
  "dimension": "B5",
  "file": "path/to/file.go",
  "line": 42,
  "severity": "crit|high|med|low",
  "claim": "one sentence: what is wrong",
  "evidence": "the exact offending code snippet",
  "trigger": "how an untrusted input or a caller reaches it",
  "suggested_fix": "the concrete change",
  "confidence": "high|med|low",
  "class": "notify|question|review"
}
```

- Candidates are unconfirmed by construction. A detector proposes; it never decides.

### 4. Verify: refute by default

- For every candidate (Family B detector output, plus the Family A judgment kinds `drift` and `stale-governed-doc`), dispatch a separate verifier Task subagent, a different instance from the detector so it is not anchored on the detector's reasoning. The verifier receives only the claim plus the file pointers and re-derives from source.
- The verifier's assigned job is to REFUTE. It tries to prove the candidate is a false positive: the input is validated upstream, an auth guard runs in middleware, the row-level-security policy exists on the table, the secret is a placeholder or test fixture, the query is parameterized, the dead symbol has a live caller or a reflection or build-tag path, the duplicated blocks diverge in behavior, the coverage gap is exercised indirectly, the flagged dependency is current or its advisory unreachable, the moved commits do not change what the doc claims.
- The burden of proof sits on the finding to survive, not on the verifier to confirm. A finding survives ONLY if the verifier cannot refute it AND can name a concrete reachable trigger path (for code) or show the claim still holds against primary source (for docs and policy).
- The verifier returns `upheld`, `refuted`, or `downgraded` with a new severity and a reachability note. Refuted findings are dropped. Downgraded findings keep the corrected severity plus the reachability proof.
- This refute-default gate is load-bearing. LLM auditors hallucinate vulnerabilities and quality nits, and a report that cries wolf trains the user to ignore it, which is worse than no audit. Only findings with demonstrated reachability reach the report.

### 5. Synthesize and rank

- Keep only `upheld` and `downgraded` findings.
- Dedupe cross-dimension overlaps. A service-role client in a client bundle is both a secret (B4) and an access-control finding (B5); merge it to one finding with both tags.
- Rank by severity times reachability, and assign each survivor a stable ID.
- Apply the baseline ratchet: diff against the prior audit note or `.stardust/baseline.json` and mark each finding new or pre-existing. New crit and high findings are the headline; pre-existing ones are standing backlog. This keeps the command adoptable on an already-dirty repo.

### 6. Write the report

Write to `docs/research/<YYYY-MM-DD-HHMM>-audit-<slug>.md`. Slug is kebab-case, 3-6 words, prefixed `audit-`. This is the convention-consistent landing zone: no `audits` collection exists, and the doc convention routes a point-in-time audit to the research type.

Frontmatter is YAML, because Stardust reads `title` and `status` as typed columns:

```yaml
---
title: <Audit title>
status: Active
date: <YYYY-MM-DD>
related: [<audited paths>, <prior audit>, <relevant specs and ADRs>]
---
```

If this audit replaces an earlier one, add `supersedes: <path>` and move the old note to `Superseded`. Documents are never deleted.

The one-line BLUF thesis stays outside the collapsibles and leads with the counts: total surviving findings broken down crit, high, med, low, and how many are new since baseline.

Body uses the research sections, each wrapped in a collapsible block so the report scans fast:

```markdown
<details>
<summary><b>Findings</b></summary>
<br>

<the section content>

</details>
```

- **Question**: the audit scope and which dimensions fired, plus the depth chosen for the ask.
- **Sources**: the code paths read, the exact deterministic commands run (`stardust check --output json`, `stardust graph`, `stardust sync --check`, the `registry` diff, dependency-scanner invocations and versions), the git short HEAD the audit ran against, and the prior audit it builds on. This makes the run reproducible.
- **Findings**: the core artifact, a severity-ranked table. Columns: `ID | dimension | severity | file:line | claim | evidence | reachability | fix | class | new?`. Only verifier-upheld and downgraded findings appear. Refuted candidates are omitted; optionally list them in a collapsed "considered and dismissed" note so the next audit does not re-flag them.
- **Recommendation**: prioritized remediation. Crit and high map to fix-now and SHOULD point at `/stardust:spec` for the remediation design; med and low map to a follow-up or the deferred list. Each item references a finding ID.
- **Open questions**: the `class=question` findings, reachable but unconfirmed, or needing a human or a backend call to settle.
- **See also**: prior audits, related specs and ADRs, and the accepted-risk or deferred list.

### 7. Regenerate the index

- Run `stardust registry` to regenerate `docs/INDEX.md` from the collections. With the Stardust post-commit hook installed, this also runs on every commit.
- If Stardust is not available, skip this step and say so.
- Write the report; do not commit it unless the user asks. The registry regenerates on the user's next commit.

### 8. Self-review

Re-read the report with fresh eyes before surfacing it, and fix inline:

- Every surviving finding carries `file:line`, an exact evidence snippet, a reachability or repro sketch, and a concrete fix. Drop anything still speculative.
- No placeholders or TBDs. No internal contradictions. IDs and severities are consistent between the table and the recommendation.
- Crit and high findings are surfaced loudly in the chat summary; the report is the durable artifact.
- The audit proposed and never mutated the audited code.

## Replaces a manual review pass

The audit is the structured alternative to an ad-hoc "look over the code" request. It keeps two disciplines:

- **Detect and verify are separate agents.** The detector proposes, the verifier refutes, and only what survives an independent adversarial pass ships. Never let one agent both find and confirm its own finding.
- **Propose, do not mutate.** The command writes a report and surfaces crit and high findings; it does not touch the audited code. Remediation is a separate, user-initiated step through `/stardust:spec` or `/stardust:execute`.

## Voice and formatting

The report is clean and neutral, not chat voice. It is technical but easy on the first read: a one-line thesis with the counts up top, defined sections, a scannable findings table, so a reader gets the headline in 30 seconds and the depth on demand. The same document must read for a human triaging, a teammate cold, and an agent fixing.

- Open with a one-line thesis (BLUF) that leads with the finding counts. No greeting, no preamble.
- Third person, objective, declarative. No persona, no "I" or "you".
- Sentence-case headers, two heading levels below the title at most.
- RFC 2119 requirement language: MUST, SHOULD, MAY. Reserve it for real requirements, not style.
- Tables and lists where scannable, prose where reasoning matters. Every section earns its place.
- **Hard bans:** no em dash or en dash, no AI-slop phrases ("furthermore", "it is worth noting", "moreover"), no emoji, no co-author or generated-by commit trailers.

## Versioning

- The audit note carries a `status` from the research closed set. It does not carry a `version` field; a single research note is replaced, not versioned.
- A later audit that replaces this one carries `supersedes` and moves the prior note to `Superseded`. Documents are never deleted. When the findings are remediated, move the note to `Archived`.

### Status vocabulary (closed set)

- Research: Active, Archived, Superseded

## Validation

- [ ] Explored with Stardust or grep and read any prior audit before writing; not a duplicate
- [ ] Family A harvested by invoking the Stardust checks, not reimplemented
- [ ] Family B detection fanned out one subagent per in-scope dimension, detectors read code and did not mutate
- [ ] Every finding verified by a separate refute-by-default agent; only upheld or downgraded findings with a reachable trigger survive
- [ ] Findings deduped across dimensions, ranked by severity times reachability, marked new or pre-existing against baseline
- [ ] Report written to `docs/research/<timestamp>-audit-<slug>.md` with YAML frontmatter and a `status` from the closed set
- [ ] Sections collapsible, the one-line thesis with counts visible outside
- [ ] Findings table carries file:line, evidence, reachability, fix, and class for every row
- [ ] No em dash, no emoji, no AI-slop in the report
- [ ] `stardust registry` was run, or its skip was noted; `docs/INDEX.md` is current
- [ ] Real timestamp from `date`, not guessed
- [ ] The audited code was not edited; findings are a proposal

## Operating rules

- Run every `date`, `stardust`, and file command from `${ROOT}`.
- Write the report this turn, then stop. Do not print a second slash command for the user to run.
- Never edit, fix, or refactor the audited code. The audit proposes; it never mutates.
- Do not commit or push; the registry regenerates on the user's next commit.
- No em dash or en dash, no AI-slop, no emoji, no co-author or generated-by trailers.
- Never create a `docs/superpowers/` or other mirror folder.
