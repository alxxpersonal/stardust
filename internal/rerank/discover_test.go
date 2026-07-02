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

// canaryServer returns an httptest server that answers a rerank canary on path
// with a well-formed results array (score = document index, so the last doc
// ranks highest), and 404s every other path. hits counts how many times path
// was called, so a test can assert a probe did or did not fire.
func canaryServer(t *testing.T, path string, hits *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if hits != nil {
			*hits++
		}
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
}

// notFoundServer returns a server that 404s everything (a runtime with no rerank
// endpoint, like Ollama today) and counts probes.
func notFoundServer(t *testing.T, hits *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits != nil {
			*hits++
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

// garbageServer returns a server that answers 200 but with a body that is not a
// valid rerank response, so it must never be adopted.
func garbageServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestResolveConfiguredSkipsDiscovery(t *testing.T) {
	probed := 0
	cand := canaryServer(t, OpenAIRerankPath, &probed)
	defer cand.Close()

	res := Resolve(context.Background(), ResolveConfig{RerankerURL: "http://configured.example"},
		[]Candidate{{Base: cand.URL, Path: OpenAIRerankPath}})

	require.Equal(t, SourceConfigured, res.Source)
	require.True(t, res.Client.Enabled())
	require.Empty(t, res.Reason)
	require.Equal(t, 0, probed, "a configured url must not trigger discovery")
}

func TestResolveNoneDisablesEverything(t *testing.T) {
	probed := 0
	cand := canaryServer(t, OpenAIRerankPath, &probed)
	defer cand.Close()

	res := Resolve(context.Background(), ResolveConfig{RerankerURL: "none"},
		[]Candidate{{Base: cand.URL, Path: OpenAIRerankPath}})

	require.Equal(t, SourceOff, res.Source)
	require.False(t, res.Client.Enabled())
	require.NotEmpty(t, res.Reason)
	require.Equal(t, 0, probed, "the none sentinel must not trigger discovery")
}

func TestResolveDiscoversFirstValidResponder(t *testing.T) {
	dead := 0
	deadSrv := notFoundServer(t, &dead)
	defer deadSrv.Close()
	good := 0
	goodSrv := canaryServer(t, OpenAIRerankPath, &good)
	defer goodSrv.Close()

	res := Resolve(context.Background(), ResolveConfig{}, []Candidate{
		{Base: deadSrv.URL, Path: OpenAIRerankPath},
		{Base: goodSrv.URL, Path: OpenAIRerankPath},
	})

	require.Equal(t, SourceDiscovered, res.Source)
	require.True(t, res.Client.Enabled())
	require.Empty(t, res.Reason)
	require.GreaterOrEqual(t, dead, 1, "the earlier candidate is probed and skipped")
	require.GreaterOrEqual(t, good, 1, "the first valid candidate is adopted")

	// The discovered client actually reranks through the adopted endpoint.
	hits := []index.Hit{{Path: "a", Score: 0.9}, {Path: "b", Score: 0.5}, {Path: "c", Score: 0.1}}
	got := res.Client.Rerank(context.Background(), "q", hits)
	require.Equal(t, "c", got[0].Path, "highest reranker score wins")
}

func TestResolveSkips404GarbageAndAdoptsValid(t *testing.T) {
	dead := notFoundServer(t, nil)
	defer dead.Close()
	noScore := garbageServer(t, `{"results":[{"index":0}]}`) // results but no numeric score
	defer noScore.Close()
	notJSON := garbageServer(t, `not json at all`)
	defer notJSON.Close()
	good := canaryServer(t, OpenAIRerankPath, nil)
	defer good.Close()

	res := Resolve(context.Background(), ResolveConfig{}, []Candidate{
		{Base: dead.URL, Path: OpenAIRerankPath},
		{Base: noScore.URL, Path: OpenAIRerankPath},
		{Base: notJSON.URL, Path: OpenAIRerankPath},
		{Base: good.URL, Path: OpenAIRerankPath},
	})
	require.Equal(t, SourceDiscovered, res.Source)
}

func TestResolveOffWhenNoRuntimeAnswers(t *testing.T) {
	dead := notFoundServer(t, nil)
	defer dead.Close()

	res := Resolve(context.Background(), ResolveConfig{}, []Candidate{
		{Base: dead.URL, Path: OpenAIRerankPath},
	})
	require.Equal(t, SourceOff, res.Source)
	require.False(t, res.Client.Enabled())
	require.NotEmpty(t, res.Reason, "an off source must carry a reason")
}

func TestResolveOllamaSeamLightsUpWhenEndpointExists(t *testing.T) {
	// The forward-compat seam: an Ollama host that serves /api/rerank is adopted
	// with zero code change the day upstream ships the endpoint.
	srv := canaryServer(t, OllamaRerankPath, nil)
	defer srv.Close()

	res := Resolve(context.Background(), ResolveConfig{}, DefaultCandidates(srv.URL))
	require.Equal(t, SourceDiscovered, res.Source)

	hits := []index.Hit{{Path: "a"}, {Path: "b"}}
	got := res.Client.Rerank(context.Background(), "q", hits)
	require.Equal(t, "b", got[0].Path, "reranked through the discovered /api/rerank endpoint")
}

func TestDefaultCandidatesOrderAndShape(t *testing.T) {
	cands := DefaultCandidates("http://localhost:11434/")
	require.Len(t, cands, 2)
	require.Equal(t, "http://localhost:11434", cands[0].Base)
	require.Equal(t, OllamaRerankPath, cands[0].Path, "the ollama seam is probed first")
	require.Equal(t, LlamaServerDefaultURL, cands[1].Base)
	require.Equal(t, OpenAIRerankPath, cands[1].Path, "the llama.cpp default is probed second")

	// An empty ollama url drops the seam, leaving only the llama.cpp default.
	only := DefaultCandidates("")
	require.Len(t, only, 1)
	require.Equal(t, LlamaServerDefaultURL, only[0].Base)
}
