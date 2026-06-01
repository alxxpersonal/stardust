package rerank

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/index"
)

func TestRerankDisabledPassThrough(t *testing.T) {
	c := New("", "")
	require.False(t, c.Enabled())
	hits := []index.Hit{{Path: "a"}, {Path: "b"}}
	require.Equal(t, hits, c.Rerank(context.Background(), "q", hits))
}

func TestRerankReorders(t *testing.T) {
	// fake reranker: score = document index, so the LAST doc ranks highest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Documents []string `json:"documents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		type res struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		}
		var out struct {
			Results []res `json:"results"`
		}
		for i := range req.Documents {
			out.Results = append(out.Results, res{Index: i, RelevanceScore: float64(i)})
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	require.True(t, c.Enabled())
	hits := []index.Hit{{Path: "a", Score: 0.9}, {Path: "b", Score: 0.5}, {Path: "c", Score: 0.1}}
	got := c.Rerank(context.Background(), "q", hits)

	require.Len(t, got, 3)
	require.Equal(t, "c", got[0].Path) // highest reranker score (index 2)
	require.Equal(t, "a", got[2].Path)
	require.Equal(t, float64(2), got[0].Score)
}

func TestRerankGracefulOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	hits := []index.Hit{{Path: "a"}, {Path: "b"}}
	require.Equal(t, hits, c.Rerank(context.Background(), "q", hits)) // unchanged on 500
}
