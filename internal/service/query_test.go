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
