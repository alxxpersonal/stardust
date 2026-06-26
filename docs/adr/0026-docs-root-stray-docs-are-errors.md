---
title: Docs root stray markdown files are convention errors
status: Proposed
version: 1
date: 2026-06-26
related:
  - internal/convention/check.go
  - internal/service/check.go
  - .stardust/collections/specs/config.toml
  - .stardust/collections/plans/config.toml
  - .stardust/collections/adr/config.toml
  - .stardust/collections/research/config.toml
---

Markdown under `docs/` MUST live in exactly one registered collection folder unless it is `docs/INDEX.md` or a template.

<details>
<summary><b>Context</b></summary>
<br>

The docs convention is collection-based. Specs, plans, ADRs, and research notes each have a canonical folder. Loose docs and mirror folders such as `docs/superpowers/` bypass the registry and confuse agents about which file is authoritative.

</details>

<details>
<summary><b>Decision</b></summary>
<br>

`stardust check --strict` reports a `stray-doc` error for markdown under `docs/` when the path is outside all registered collection folders.

The rule exempts `docs/INDEX.md` and markdown under `docs/templates/`.

</details>

<details>
<summary><b>Consequences</b></summary>
<br>

- The checker enforces the no-mirror convention instead of relying on docs prose.
- Agents get one canonical collection location per durable doc.
- Repos with ad hoc docs under `docs/` must move them into a collection or templates.

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Warn instead of error. Rejected because the request requires strict enforcement.
- Hardcode only the default four folders. Rejected because registered collection configs are the authority.
- Exempt all nested unknown folders. Rejected because mirror folders are the core failure mode.

</details>

<details>
<summary><b>References</b></summary>
<br>

- `docs/specs/2026-06-26-1849-stardust-hardening.md`
- `internal/convention/check.go`

</details>
