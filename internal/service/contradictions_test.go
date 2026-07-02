package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/temporal"
)

// gitCommit initializes a git repo at root (idempotent) and commits everything.
func gitCommit(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		for _, args := range [][]string{
			{"init"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
			{"config", "commit.gpgsign", "false"},
		} {
			run := exec.Command("git", append([]string{"-C", root}, args...)...)
			out, err := run.CombinedOutput()
			require.NoError(t, err, "git %v: %s", args, string(out))
		}
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", "snapshot", "--allow-empty"}} {
		run := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := run.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}
}

func writeNote(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// prepareVault seeds notes, commits them, and indexes with the given embedder,
// returning a ready service scanning all notes.
func prepareVault(t *testing.T, available bool, notes map[string]string) *Service {
	t.Helper()
	svc, root := newServiceWith(t, &fakeEmbedder{available: available}, "")
	for rel, content := range notes {
		writeNote(t, root, rel, content)
	}
	gitCommit(t, root)
	_, err := svc.Index(context.Background(), "")
	require.NoError(t, err)
	return svc
}

func pathSetOf(cands []Contradiction) map[string]bool {
	out := map[string]bool{}
	for _, c := range cands {
		out[c.NoteA] = true
		out[c.NoteB] = true
	}
	return out
}

func TestContradictionsRealConflictFires(t *testing.T) {
	svc := prepareVault(t, true, map[string]string{
		"decisions/db.md":  "---\ntitle: DB decision\n---\n# DB\nWe decided to use Postgres for the primary database.",
		"notes/switch.md":  "---\ntitle: Switch\n---\n# Switch\nPostgres is no longer our database; we moved off it.",
		"notes/weather.md": "---\ntitle: Weather\n---\n# Weather\nThe sky was blue over Warsaw all week.",
	})

	res, err := svc.Contradictions(context.Background(), ContradictionOptions{All: true})
	require.NoError(t, err)
	require.Len(t, res.Candidates, 1, "exactly one candidate, symmetric dedupe collapses (A,B) and (B,A)")

	c := res.Candidates[0]
	require.Equal(t, map[string]bool{"decisions/db.md": true, "notes/switch.md": true}, pathSetOf(res.Candidates))
	require.Contains(t, c.SharedTerms, "postgres")
	require.Equal(t, "no longer", c.Cue, "the reversal side carries the discriminating cue")
	require.Equal(t, RetrievalHybridSemantic, c.RetrievalMode)
	require.Contains(t, res.Markdown, "candidate, not a verdict")
	require.Contains(t, res.Markdown, "Possible contradiction (review)")
}

func TestContradictionsAgreementDoesNotFire(t *testing.T) {
	svc := prepareVault(t, true, map[string]string{
		"a.md": "---\ntitle: A\n---\n# A\nWe decided to use Postgres for the primary database.",
		"b.md": "---\ntitle: B\n---\n# B\nWe chose Postgres as the canonical primary database.",
	})

	res, err := svc.Contradictions(context.Background(), ContradictionOptions{All: true})
	require.NoError(t, err)
	require.Empty(t, res.Candidates, "two agreeing assertions must not fire; similarity is inert without polarity XOR")
}

func TestContradictionsOffSubjectDoesNotFire(t *testing.T) {
	svc := prepareVault(t, true, map[string]string{
		"a.md": "---\ntitle: A\n---\n# A\nWe will use Postgres for the primary database.",
		"b.md": "---\ntitle: B\n---\n# B\nRedux is no longer our frontend state library.",
	})

	res, err := svc.Contradictions(context.Background(), ContradictionOptions{All: true})
	require.NoError(t, err)
	require.Empty(t, res.Candidates, "different subjects fall below the shared-term floor even with a reversal cue")
}

func TestContradictionsSupersededExcluded(t *testing.T) {
	svc := prepareVault(t, true, map[string]string{
		"decisions/db.md": "---\ntitle: DB decision\n---\n# DB\nWe decided to use Postgres for the primary database.",
		"notes/switch.md": "---\ntitle: Switch\nsuperseded_by: decisions/db.md\nvalid_to: 2026-01-01\n---\n# Switch\nPostgres is no longer our database.",
	})

	res, err := svc.Contradictions(context.Background(), ContradictionOptions{All: true})
	require.NoError(t, err)
	require.Empty(t, res.Candidates, "a superseded record is the sanctioned change-your-mind path, never a contradiction")
}

func TestContradictionsCapAndDedupe(t *testing.T) {
	notes := map[string]string{}
	subjects := []string{"postgres", "redis", "kafka", "elasticsearch", "mongodb", "cassandra", "rabbitmq"}
	for _, subj := range subjects {
		notes["decide-"+subj+".md"] = "---\ntitle: decide " + subj + "\n---\n# d\nWe decided to use " + subj + " for the primary " + subj + " store."
		notes["drop-"+subj+".md"] = "---\ntitle: drop " + subj + "\n---\n# d\nThe " + subj + " primary " + subj + " store is no longer used."
	}
	svc := prepareVault(t, true, notes)

	res, err := svc.Contradictions(context.Background(), ContradictionOptions{All: true, Limit: 3})
	require.NoError(t, err)
	require.Len(t, res.Candidates, 3, "hard cap holds as a ceiling")

	seen := map[string]bool{}
	for _, c := range res.Candidates {
		key := unorderedPairKey(c.NoteA, c.NoteB)
		require.False(t, seen[key], "symmetric pairs are deduped")
		seen[key] = true
		require.NotEqual(t, c.NoteA, c.NoteB, "different notes only")
	}
}

func TestContradictionsFTSOnlyDegradationIsLoud(t *testing.T) {
	notes := map[string]string{
		"decisions/db.md": "---\ntitle: DB decision\n---\n# DB\nWe decided to use Postgres for the primary database.",
		"notes/switch.md": "---\ntitle: Switch\n---\n# Switch\nPostgres is no longer our database; we moved off it.",
	}

	up := prepareVault(t, true, notes)
	upRes, err := up.Contradictions(context.Background(), ContradictionOptions{All: true})
	require.NoError(t, err)
	require.Equal(t, RetrievalHybridSemantic, upRes.RetrievalMode)
	require.Empty(t, upRes.RetrievalReason)

	down := prepareVault(t, false, notes)
	downRes, err := down.Contradictions(context.Background(), ContradictionOptions{All: true})
	require.NoError(t, err)
	require.Equal(t, RetrievalFTSOnly, downRes.RetrievalMode)
	require.NotEmpty(t, downRes.RetrievalReason, "degradation is announced, never silent")
	require.Contains(t, downRes.Markdown, "fts-only")
	require.Len(t, downRes.Candidates, 1, "fts recall still finds the same-subject opposing note")
}

func TestContradictionsCursorIndependentOfDigest(t *testing.T) {
	svc := prepareVault(t, true, map[string]string{
		"decisions/db.md": "---\ntitle: DB decision\n---\n# DB\nWe decided to use Postgres for the primary database.",
		"notes/switch.md": "---\ntitle: Switch\n---\n# Switch\nPostgres is no longer our database; we moved off it.",
	})
	ctx := context.Background()

	// advancing the digest cursor to HEAD must not blind the contradiction scan,
	// which keys off its own cursor (contradictionCursorKey).
	_, err := svc.Digest(ctx, "", true)
	require.NoError(t, err)

	res, err := svc.Contradictions(ctx, ContradictionOptions{})
	require.NoError(t, err)
	require.Len(t, res.Candidates, 1, "the contradiction cursor is independent of the digest cursor")

	// only contradictions --advance moves the contradiction cursor.
	_, err = svc.Contradictions(ctx, ContradictionOptions{Advance: true})
	require.NoError(t, err)
	after, err := svc.Contradictions(ctx, ContradictionOptions{})
	require.NoError(t, err)
	require.Empty(t, after.Candidates, "after advancing to HEAD the incremental scan sees nothing new")
}

func TestContradictionsPairGatesPure(t *testing.T) {
	// jaccard measures shared-subject overlap over de-stopworded tokens.
	j, shared := jaccard(contentTokens("we decided to use postgres for storage"),
		contentTokens("postgres storage is no longer used"))
	require.Greater(t, j, contradictionJaccardFloor)
	require.Contains(t, shared, "postgres")
	require.Contains(t, shared, "storage")

	// polarity XOR: an assertion anchor only pairs with a reversal-bearing line.
	anchor, ok := anchorLinesOne(t, "We decided to use postgres for storage.")
	require.False(t, anchor.Reversal)
	require.True(t, ok)
	_, _, _, matched := bestOpposingLine("postgres storage is no longer used here.", anchor, contentTokens(anchor.Line))
	require.True(t, matched, "opposite polarity, shared subject -> a pair")
	_, _, _, sameSide := bestOpposingLine("postgres storage is our chosen default.", anchor, contentTokens(anchor.Line))
	require.False(t, sameSide, "same polarity (both assert) -> no pair")

	// symmetric dedupe collapses (A,B) and (B,A).
	deduped := rankDedupeCap([]scoredCandidate{
		{cand: Contradiction{NoteA: "a.md", NoteB: "b.md"}, combined: 0.5},
		{cand: Contradiction{NoteA: "b.md", NoteB: "a.md"}, combined: 0.4},
	}, 10)
	require.Len(t, deduped, 1)
}

// anchorLinesOne returns the single anchor parsed from a one-line body.
func anchorLinesOne(t *testing.T, line string) (temporal.Anchor, bool) {
	t.Helper()
	got := anchorLines(line)
	if len(got) == 0 {
		return temporal.Anchor{}, false
	}
	return got[0], true
}
