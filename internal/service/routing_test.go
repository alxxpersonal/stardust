package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
)

// --- routePlanFor: pure decision-rule unit tests (one per ADR 0042 branch) ---

func rm(name, desc string, keywords []string, vec []float32) routeMount {
	return routeMount{name: name, description: desc, keywords: keywords, descVec: vec}
}

func TestRoutePlanNoMountSearchesNothingToRoute(t *testing.T) {
	p := routePlanFor(nil, nil, "anything", nil)
	require.Equal(t, RoutingAll, p.mode)
	require.Empty(t, p.search)
	require.Empty(t, p.skipped)
}

func TestRoutePlanSingleMountByteIdentical(t *testing.T) {
	// One mount never routes: mode all, no skip, whatever metadata it carries.
	p := routePlanFor([]routeMount{rm("notion", "team wiki", []string{"wiki"}, []float32{1, 0})}, nil, "wiki", []float32{1, 0})
	require.Equal(t, RoutingAll, p.mode)
	require.Equal(t, []string{"notion"}, p.search)
	require.Empty(t, p.skipped)
}

func TestRoutePlanExplicitListScopesDirectly(t *testing.T) {
	ms := []routeMount{rm("a", "", nil, nil), rm("b", "", nil, nil), rm("c", "", nil, nil)}
	p := routePlanFor(ms, []string{"a"}, "unrelated query", nil)
	require.Equal(t, RoutingRouted, p.mode)
	require.Equal(t, []string{"a"}, p.search)
	require.Equal(t, []string{"b", "c"}, p.skipped)
}

func TestRoutePlanExplicitListIsCaseInsensitiveAndCanonical(t *testing.T) {
	ms := []routeMount{rm("Notion", "", nil, nil), rm("postgres", "", nil, nil)}
	p := routePlanFor(ms, []string{"notion"}, "q", nil)
	require.Equal(t, RoutingRouted, p.mode)
	require.Equal(t, []string{"Notion"}, p.search) // canonical name preserved
	require.Equal(t, []string{"postgres"}, p.skipped)
}

func TestRoutePlanExplicitListCoveringAllFallsBack(t *testing.T) {
	ms := []routeMount{rm("a", "", nil, nil), rm("b", "", nil, nil)}
	p := routePlanFor(ms, []string{"a", "b"}, "q", nil)
	require.Equal(t, RoutingFallback, p.mode)
	require.ElementsMatch(t, []string{"a", "b"}, p.search)
	require.Empty(t, p.skipped)
}

func TestRoutePlanExplicitListAllUnknownFallsBack(t *testing.T) {
	ms := []routeMount{rm("a", "", nil, nil), rm("b", "", nil, nil)}
	p := routePlanFor(ms, []string{"nonexistent"}, "q", nil)
	require.Equal(t, RoutingFallback, p.mode) // never scope to zero on a typo
	require.ElementsMatch(t, []string{"a", "b"}, p.search)
}

func TestRoutePlanNameMentionScopes(t *testing.T) {
	ms := []routeMount{rm("notion", "", nil, nil), rm("postgres", "", nil, nil), rm("gmail", "", nil, nil)}
	p := routePlanFor(ms, nil, "search notion for the launch plan", nil)
	require.Equal(t, RoutingRouted, p.mode)
	require.Equal(t, []string{"notion"}, p.search)
	require.ElementsMatch(t, []string{"postgres", "gmail"}, p.skipped)
}

func TestRoutePlanSemanticMatchAboveThresholdRoutes(t *testing.T) {
	ms := []routeMount{
		rm("wiki", "team wiki notes", nil, []float32{1, 0}),  // cosine 1.0 with query
		rm("db", "postgres analytics", nil, []float32{0, 1}), // cosine 0.0 with query
	}
	p := routePlanFor(ms, nil, "q", []float32{1, 0})
	require.Equal(t, RoutingRouted, p.mode)
	require.Equal(t, []string{"wiki"}, p.search)
	require.Equal(t, []string{"db"}, p.skipped)
}

func TestRoutePlanSemanticNoneMatchFallsBack(t *testing.T) {
	ms := []routeMount{
		rm("wiki", "d", nil, []float32{0, 1}),
		rm("db", "d", nil, []float32{-1, 0}),
	}
	p := routePlanFor(ms, nil, "q", []float32{1, 0})
	require.Equal(t, RoutingFallback, p.mode) // empty subset -> search all, never zero
	require.ElementsMatch(t, []string{"wiki", "db"}, p.search)
}

func TestRoutePlanSemanticAllMatchFallsBack(t *testing.T) {
	ms := []routeMount{
		rm("wiki", "d", nil, []float32{1, 0}),
		rm("db", "d", nil, []float32{1, 0}),
	}
	p := routePlanFor(ms, nil, "q", []float32{1, 0})
	require.Equal(t, RoutingFallback, p.mode) // subset == full set -> report fallback, searches all
	require.ElementsMatch(t, []string{"wiki", "db"}, p.search)
	require.Empty(t, p.skipped)
}

func TestRoutePlanMetadataLessMountAlwaysSearched(t *testing.T) {
	// wiki matches semantically, db does not, plain has no metadata -> always in.
	ms := []routeMount{
		rm("wiki", "d", nil, []float32{1, 0}),
		rm("db", "d", nil, []float32{0, 1}),
		rm("plain", "", nil, nil),
	}
	p := routePlanFor(ms, nil, "q", []float32{1, 0})
	require.Equal(t, RoutingRouted, p.mode)
	require.ElementsMatch(t, []string{"wiki", "plain"}, p.search) // plain never excludable
	require.Equal(t, []string{"db"}, p.skipped)
}

func TestRoutePlanSemanticDescribedButUnvectoredStaysIn(t *testing.T) {
	// A described mount whose description could not be embedded cannot be scored,
	// so it cannot be confidently excluded: keep it.
	ms := []routeMount{
		rm("wiki", "d", nil, []float32{1, 0}),
		rm("db", "d", nil, nil), // described, but no vector
	}
	p := routePlanFor(ms, nil, "q", []float32{1, 0})
	require.Equal(t, RoutingFallback, p.mode) // both kept -> full set -> fallback
	require.ElementsMatch(t, []string{"wiki", "db"}, p.search)
}

func TestRoutePlanLexicalMatchInFTSOnlyRoutes(t *testing.T) {
	ms := []routeMount{
		rm("wiki", "team wiki and meeting notes", []string{"wiki", "meeting"}, nil),
		rm("db", "postgres analytics warehouse", []string{"postgres", "sql"}, nil),
	}
	// queryVec nil -> fts-only lexical routing; "postgres" hits db keywords only.
	p := routePlanFor(ms, nil, "how big is the postgres table", nil)
	require.Equal(t, RoutingRouted, p.mode)
	require.Equal(t, []string{"db"}, p.search)
	require.Equal(t, []string{"wiki"}, p.skipped)
}

func TestRoutePlanLexicalNoHitFallsBack(t *testing.T) {
	ms := []routeMount{
		rm("wiki", "team wiki", []string{"wiki"}, nil),
		rm("db", "postgres", []string{"postgres"}, nil),
	}
	p := routePlanFor(ms, nil, "quarterly revenue projections", nil)
	require.Equal(t, RoutingFallback, p.mode)
	require.ElementsMatch(t, []string{"wiki", "db"}, p.search)
}

func TestRoutePlanNoMetadataAnywhereStaysQuietAll(t *testing.T) {
	// Two mounts, neither describes itself -> nothing to route on -> mode all,
	// byte-identical to a pre-metadata workspace (no routing line will render).
	ms := []routeMount{rm("a", "", nil, nil), rm("b", "", nil, nil)}
	p := routePlanFor(ms, nil, "anything at all", nil)
	require.Equal(t, RoutingAll, p.mode)
	require.ElementsMatch(t, []string{"a", "b"}, p.search)
	require.Empty(t, p.skipped)
}

// --- lexical + tokenize helpers ---

func TestLexicalMatch(t *testing.T) {
	m := rm("notion", "team wiki and project notes", []string{"meeting-notes", "roadmap"}, nil)
	require.True(t, lexicalMatch("open the roadmap", m))  // keyword hit
	require.True(t, lexicalMatch("what is in notion", m)) // name hit
	require.True(t, lexicalMatch("project status", m))    // description hit
	require.False(t, lexicalMatch("kubernetes deployment", m))
	require.False(t, lexicalMatch("", m))
}

// --- QueryMounts integration: seeded mounts, one clearly-scoped, one ambiguous ---

func seedMount(t *testing.T, root, name, toml string) {
	t.Helper()
	dir := filepath.Join(config.Layout{Root: root}.Mounts(), name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(toml), 0o644))
}

func TestQueryMountsRoutesScopedQueryAndFallsBackOnAmbiguous(t *testing.T) {
	// fts-only so the outcome is deterministic regardless of a live Ollama.
	svc, root := newServiceWith(t, &fakeEmbedder{available: false}, "")
	seedQueryNotes(t, root)
	_, err := svc.Index(context.Background(), "")
	require.NoError(t, err)

	// "true" exits immediately: the connector fails fast and is gracefully
	// skipped, which does not affect the routing decision under test.
	seedMount(t, root, "notion", "command = \"true\"\n")
	seedMount(t, root, "postgres", "command = \"true\"\n")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Clearly scoped: the query names one mount -> routed to it, the other skipped.
	// The vault terms overlap the seeded notes so the always-searched vault returns.
	scoped, err := svc.QueryMounts(ctx, "notion retrieval index search", 10, nil)
	require.NoError(t, err)
	require.Equal(t, RoutingRouted, scoped.RoutingMode)
	require.Equal(t, []string{"notion"}, scoped.MountsSearched)
	require.Equal(t, []string{"postgres"}, scoped.MountsSkipped)
	require.Equal(t, RetrievalFTSOnly, scoped.RetrievalMode) // routing travels beside retrieval
	requireHasVaultHit(t, scoped)                            // vault is always searched

	// Ambiguous: no name mention, no metadata to route on -> searches everything.
	ambiguous, err := svc.QueryMounts(ctx, "retrieval index search", 10, nil)
	require.NoError(t, err)
	require.Equal(t, RoutingAll, ambiguous.RoutingMode)
	require.ElementsMatch(t, []string{"notion", "postgres"}, ambiguous.MountsSearched)
	require.Empty(t, ambiguous.MountsSkipped)
	requireHasVaultHit(t, ambiguous)
}

func TestQueryMountsNoMountsIsQuietAll(t *testing.T) {
	svc, root := newServiceWith(t, &fakeEmbedder{available: false}, "")
	seedQueryNotes(t, root)
	_, err := svc.Index(context.Background(), "")
	require.NoError(t, err)

	res, err := svc.QueryMounts(context.Background(), "retrieval index search", 10, nil)
	require.NoError(t, err)
	require.Equal(t, RoutingAll, res.RoutingMode) // no mounts -> byte-identical, no routing
	require.Empty(t, res.MountsSearched)
	require.Empty(t, res.MountsSkipped)
	requireHasVaultHit(t, res)
}

func requireHasVaultHit(t *testing.T, res MountQueryResult) {
	t.Helper()
	for _, h := range res.Hits {
		if h.Source == "vault" {
			return
		}
	}
	t.Fatalf("expected at least one vault hit, got %d hits: %+v", len(res.Hits), res.Hits)
}
