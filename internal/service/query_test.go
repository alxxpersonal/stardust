package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/rerank"
)

func seedQueryNotes(t *testing.T, root string) {
	t.Helper()
	notes := map[string]string{
		"alpha.md": "---\ntitle: Alpha\n---\n# Alpha\nalpha beta gamma retrieval index search",
		"beta.md":  "---\ntitle: Beta\n---\n# Beta\nbeta gamma retrieval index search delta",
		"gamma.md": "---\ntitle: Gamma\n---\n# Gamma\ngamma retrieval index search delta epsilon",
	}
	for name, content := range notes {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
}

func TestQueryRetrievalModeHybridSemantic(t *testing.T) {
	svc, root := newServiceWith(t, &fakeEmbedder{available: true}, "")
	seedQueryNotes(t, root)
	_, err := svc.Index(context.Background(), "")
	require.NoError(t, err)

	res, err := svc.Query(context.Background(), "alpha beta gamma", 10)
	require.NoError(t, err)
	require.Equal(t, "hybrid-semantic", res.RetrievalMode)
	require.Empty(t, res.RetrievalReason)
}

func TestQueryRetrievalModeFTSOnly(t *testing.T) {
	svc, root := newServiceWith(t, &fakeEmbedder{available: false}, "")
	seedQueryNotes(t, root)
	_, err := svc.Index(context.Background(), "")
	require.NoError(t, err)

	res, err := svc.Query(context.Background(), "alpha beta gamma", 10)
	require.NoError(t, err)
	require.Equal(t, "fts-only", res.RetrievalMode)
	require.NotEmpty(t, res.RetrievalReason) // degradation is announced, never silent
}

// reversingReranker returns a reranker server that scores later documents higher,
// reversing the fused order it receives.
func reversingReranker(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Documents []string `json:"documents"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		type result struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		}
		out := struct {
			Results []result `json:"results"`
		}{}
		for i := range req.Documents {
			out.Results = append(out.Results, result{Index: i, RelevanceScore: float64(i)})
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(out))
	}))
}

func TestQueryRerankReorders(t *testing.T) {
	ctx := context.Background()

	plain, rootA := newServiceWith(t, &fakeEmbedder{available: true}, "")
	seedQueryNotes(t, rootA)
	_, err := plain.Index(ctx, "")
	require.NoError(t, err)
	base, err := plain.Query(ctx, "retrieval index search", 10)
	require.NoError(t, err)
	require.False(t, base.Reranked)
	require.GreaterOrEqual(t, len(base.Hits), 2)

	srv := reversingReranker(t)
	defer srv.Close()
	reranked, rootB := newServiceWith(t, &fakeEmbedder{available: true}, srv.URL)
	seedQueryNotes(t, rootB)
	_, err = reranked.Index(ctx, "")
	require.NoError(t, err)
	rres, err := reranked.Query(ctx, "retrieval index search", 10)
	require.NoError(t, err)
	require.True(t, rres.Reranked)                                // rerank reordered the top-k
	require.NotEqual(t, hitPaths(base.Hits), hitPaths(rres.Hits)) // order actually changed
	require.Equal(t, "hybrid-semantic", rres.RetrievalMode)
}

func TestQueryRerankUnreachable(t *testing.T) {
	ctx := context.Background()
	// nothing is listening on this address, so Rerank degrades to identity.
	svc, root := newServiceWith(t, &fakeEmbedder{available: true}, "http://127.0.0.1:0")
	seedQueryNotes(t, root)
	_, err := svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.Query(ctx, "retrieval index search", 10)
	require.NoError(t, err)
	require.False(t, res.Reranked) // unreachable reranker leaves the fused order intact
}

// TestQueryRerankSourceConfigured asserts an explicit reranker_url is announced
// as the configured source and used verbatim, byte-identical to today.
func TestQueryRerankSourceConfigured(t *testing.T) {
	ctx := context.Background()
	srv := reversingReranker(t)
	defer srv.Close()

	svc, root := newServiceWith(t, &fakeEmbedder{available: true}, srv.URL)
	seedQueryNotes(t, root)
	_, err := svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.Query(ctx, "retrieval index search", 10)
	require.NoError(t, err)
	require.Equal(t, string(rerank.SourceConfigured), res.RerankSource)
	require.Empty(t, res.RerankReason)
	require.True(t, res.Reranked)
}

// TestQueryRerankSourceDiscovered asserts that with no reranker_url set, the
// service discovers a local runtime via the injected candidate list, reranks
// through it, and announces the discovered source.
func TestQueryRerankSourceDiscovered(t *testing.T) {
	ctx := context.Background()
	srv := reversingReranker(t)
	defer srv.Close()

	svc, root := newServiceWith(t, &fakeEmbedder{available: true}, "")
	svc.rerankProbes = []rerank.Candidate{{Base: srv.URL, Path: rerank.OpenAIRerankPath}}
	seedQueryNotes(t, root)
	_, err := svc.Index(ctx, "")
	require.NoError(t, err)

	base, err := svc.Query(ctx, "retrieval index search", 10)
	require.NoError(t, err)
	require.Equal(t, string(rerank.SourceDiscovered), base.RerankSource)
	require.Empty(t, base.RerankReason)
	require.True(t, base.Reranked) // the discovered runtime reordered the top-k
}

// TestQueryRerankSourceOff asserts that with no reranker_url set and no runtime
// discoverable, the reranker is announced off with a reason and the hybrid order
// is returned unchanged, never failing the query.
func TestQueryRerankSourceOff(t *testing.T) {
	ctx := context.Background()
	svc, root := newServiceWith(t, &fakeEmbedder{available: true}, "")
	// A dead candidate that never answers: discovery finds nothing.
	svc.rerankProbes = []rerank.Candidate{{Base: "http://127.0.0.1:0", Path: rerank.OpenAIRerankPath}}
	seedQueryNotes(t, root)
	_, err := svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.Query(ctx, "retrieval index search", 10)
	require.NoError(t, err)
	require.Equal(t, string(rerank.SourceOff), res.RerankSource)
	require.NotEmpty(t, res.RerankReason) // off is announced with a reason, never silent
	require.False(t, res.Reranked)
}

// TestQueryRerankNoneSentinelDisables asserts the reranker_url "none" sentinel
// hard-disables both the reranker and discovery: the source is off with a reason
// and no probe fires even if a runtime is reachable.
func TestQueryRerankNoneSentinelDisables(t *testing.T) {
	ctx := context.Background()
	srv := reversingReranker(t)
	defer srv.Close()

	svc, root := newServiceWith(t, &fakeEmbedder{available: true}, "none")
	// A reachable candidate that must never be probed because the sentinel wins.
	svc.rerankProbes = []rerank.Candidate{{Base: srv.URL, Path: rerank.OpenAIRerankPath}}
	seedQueryNotes(t, root)
	_, err := svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.Query(ctx, "retrieval index search", 10)
	require.NoError(t, err)
	require.Equal(t, string(rerank.SourceOff), res.RerankSource)
	require.NotEmpty(t, res.RerankReason)
	require.False(t, res.Reranked)
}
