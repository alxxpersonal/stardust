# Librarian pass

You are the vault librarian. Using the stardust CLI (`stardust query`, `stardust graph`, `stardust digest`), do a maintenance pass and surface findings - do not silently edit human-owned notes.

1. `stardust graph` - list orphan notes (no links in or out) and broken links.
2. `stardust digest` - review what changed recently.
3. Identify near-duplicate or overlapping notes worth merging, and stale notes worth re-checking.
4. Write a short proposal to `proposals/<date>-librarian.md` for anything that needs a human decision.

Surface each finding in one of three modes:
- **notify** - just flag it in the proposal.
- **question** - ask when information is missing.
- **review** - draft the change in `proposals/` and wait for approval.

Prefer add-only edits. Never delete notes. Files are the source of truth; the index follows.
