package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// --- Source resolution ---

// Source announces where the active reranker endpoint came from, extending the
// loud-degradation rule (ADR 0016) to the rerank stage so a consumer never
// mistakes "no runtime found" for "a reranker ran and kept the order."
type Source string

const (
	// SourceConfigured means an explicit reranker_url was used verbatim.
	SourceConfigured Source = "configured"
	// SourceDiscovered means discovery adopted a local runtime.
	SourceDiscovered Source = "discovered"
	// SourceOff means no reranker is active; Reason on the Resolution explains why.
	SourceOff Source = "off"
)

// Rerank contract paths. The /v1/rerank path is the llama.cpp / Jina / Cohere
// contract internal/rerank already speaks; /api/rerank is the forward-compat
// Ollama seam that carries no endpoint today.
const (
	OpenAIRerankPath = "/v1/rerank"
	OllamaRerankPath = "/api/rerank"
)

// LlamaServerDefaultURL is the llama.cpp llama-server default base URL, which
// serves /v1/rerank when launched with --reranking.
const LlamaServerDefaultURL = "http://localhost:8080"

// disableSentinel, set as reranker_url, hard-disables both the reranker and
// discovery for users who do not want localhost probing.
const disableSentinel = "none"

// Off-source reasons.
const (
	reasonDisabled  = "reranking disabled (reranker_url set to none)"
	reasonNoRuntime = "no reranker configured and no local runtime discovered"
)

// probeTimeout bounds each discovery canary so a slow or black-holed candidate
// never stalls the first query that resolves the source.
const probeTimeout = 3 * time.Second

// probeClient is a dedicated short-timeout client for discovery canaries, kept
// separate from the reranker's generous 30s request timeout.
var probeClient = &http.Client{Timeout: probeTimeout}

// Candidate is one endpoint discovery probes with a two-document canary rerank.
type Candidate struct {
	Base string // base URL, e.g. http://localhost:8080
	Path string // rerank endpoint path, e.g. /v1/rerank
}

// DefaultCandidates is the ordered discovery probe list: the already-configured
// Ollama host first (the forward-compat /api/rerank seam, which yields nothing
// today), then the llama.cpp llama-server default (/v1/rerank, which works
// today). An empty ollamaURL drops the seam. The Ollama-first order means the
// reuse-Ollama future activates for free the day upstream ships /api/rerank.
func DefaultCandidates(ollamaURL string) []Candidate {
	out := make([]Candidate, 0, 2)
	if base := strings.TrimRight(strings.TrimSpace(ollamaURL), "/"); base != "" {
		out = append(out, Candidate{Base: base, Path: OllamaRerankPath})
	}
	out = append(out, Candidate{Base: LlamaServerDefaultURL, Path: OpenAIRerankPath})
	return out
}

// ResolveConfig carries the reranker knobs source resolution reads.
type ResolveConfig struct {
	RerankerURL   string
	RerankerModel string
}

// Resolution is the outcome of source resolution: a ready Client (disabled when
// off, so Rerank is always callable), the announced Source, and a Reason that is
// populated only when Source is off.
type Resolution struct {
	Client *Client
	Source Source
	Reason string
}

// Resolve picks the reranker source with the fallback chain configured >
// discovered > none. A real reranker_url is used verbatim (the override and
// escape hatch); the sentinel "none" hard-disables the reranker and discovery;
// an empty reranker_url probes the candidate list once and adopts the first
// endpoint that answers a canary with a well-formed, scored results array. It
// never fails: with no source it returns a disabled Client and Source off with a
// reason, and the caller returns hybrid order unchanged.
func Resolve(ctx context.Context, cfg ResolveConfig, candidates []Candidate) Resolution {
	url := strings.TrimSpace(cfg.RerankerURL)
	switch {
	case strings.EqualFold(url, disableSentinel):
		return Resolution{Client: New("", cfg.RerankerModel), Source: SourceOff, Reason: reasonDisabled}
	case url != "":
		return Resolution{Client: New(url, cfg.RerankerModel), Source: SourceConfigured}
	}
	for _, c := range candidates {
		if probeCandidate(ctx, c) {
			return Resolution{Client: newClient(c.Base, c.Path, cfg.RerankerModel), Source: SourceDiscovered}
		}
	}
	return Resolution{Client: New("", cfg.RerankerModel), Source: SourceOff, Reason: reasonNoRuntime}
}

// probeCandidate sends a two-document canary rerank (documents "a" and "b"
// against query "a") and reports whether the endpoint returned a well-formed
// results array carrying a numeric relevance score. Any transport error,
// non-200, decode failure, empty results, or a missing score is a miss, so a
// server that answers 200 with the wrong body is never adopted.
func probeCandidate(ctx context.Context, c Candidate) bool {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	body, err := json.Marshal(map[string]any{"query": "a", "documents": []string{"a", "b"}})
	if err != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.Base, "/")+c.Path, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := probeClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	var out struct {
		Results []struct {
			Index          int      `json:"index"`
			RelevanceScore *float64 `json:"relevance_score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Results) == 0 {
		return false
	}
	for _, r := range out.Results {
		if r.RelevanceScore == nil {
			return false
		}
	}
	return true
}
