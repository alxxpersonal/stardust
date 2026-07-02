// Package service is Stardust's core: one library over a vault that every
// surface (CLI, HTTP API, MCP server) calls. Capability parity is structural -
// each surface is a thin caller of these methods, so none can do anything the
// others cannot.
package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/cron"
	"github.com/alxxpersonal/stardust/internal/embed"
	"github.com/alxxpersonal/stardust/internal/graph"
	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/rerank"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// embedder is the embedding capability the service depends on: availability
// probing, batch embedding, and the model name. *embed.Client satisfies it; tests
// inject a fake so the vector path runs without a live Ollama.
type embedder interface {
	Available(ctx context.Context) bool
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Model() string
}

// Service is the open core over one vault: the index, embedder, and reranker.
type Service struct {
	Layout config.Layout
	Config config.Config
	store  *index.Store
	embed  embedder

	// The reranker source resolves lazily, once per service lifetime, on the
	// first Query or Status that needs it, so index and other commands pay no
	// discovery probe. The fallback chain is configured > discovered > none;
	// rerankProbes overrides the discovery candidate list in tests (nil uses
	// rerank.DefaultCandidates over the configured Ollama host).
	rerankMu     sync.Mutex
	rerankReady  bool
	rerankClient *rerank.Client
	rerankSource rerank.Source
	rerankReason string
	rerankProbes []rerank.Candidate
}

// Open resolves the vault containing start (walking up for .stardust), loads its
// config, opens the index, and returns a ready Service. Callers must Close.
func Open(ctx context.Context, start string) (*Service, error) {
	root, err := config.FindRoot(start)
	if err != nil {
		return nil, err
	}
	layout := config.Layout{Root: root}
	cfg, err := config.Load(layout.Config())
	if err != nil {
		return nil, err
	}
	store, err := index.Open(ctx, layout.DB())
	if err != nil {
		return nil, err
	}
	return &Service{
		Layout: layout,
		Config: cfg,
		store:  store,
		embed:  embed.New(cfg.OllamaURL, cfg.EmbedModel),
	}, nil
}

// Close releases the index handle.
func (s *Service) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

// --- Config mutation ---

// SetConfig persists cfg to the vault's config.toml and updates the live
// service, rebuilding the embed client and invalidating the cached reranker
// resolution so later reads use the new model, Ollama URL, and reranker settings
// without reopening the service. The reranker source re-resolves on the next
// Query or Status.
func (s *Service) SetConfig(cfg config.Config) error {
	if err := config.Save(s.Layout.Config(), cfg); err != nil {
		return err
	}
	s.Config = cfg
	s.embed = embed.New(cfg.OllamaURL, cfg.EmbedModel)
	s.rerankMu.Lock()
	s.rerankReady = false
	s.rerankMu.Unlock()
	return nil
}

// resolveRerank returns the reranker client, its announced source, and an off
// reason, resolving the fallback chain (configured > discovered > none) once and
// caching the outcome for the service lifetime. Discovery probes run at most
// once, only when reranker_url is empty, and never fail: an unreachable or absent
// runtime resolves to a disabled client and an off source with a reason.
func (s *Service) resolveRerank(ctx context.Context) (*rerank.Client, rerank.Source, string) {
	s.rerankMu.Lock()
	defer s.rerankMu.Unlock()
	if !s.rerankReady {
		candidates := s.rerankProbes
		if candidates == nil {
			candidates = rerank.DefaultCandidates(s.Config.OllamaURL)
		}
		res := rerank.Resolve(ctx, rerank.ResolveConfig{
			RerankerURL:   s.Config.RerankerURL,
			RerankerModel: s.Config.RerankerModel,
		}, candidates)
		s.rerankClient = res.Client
		s.rerankSource = res.Source
		s.rerankReason = res.Reason
		s.rerankReady = true
	}
	return s.rerankClient, s.rerankSource, s.rerankReason
}

// --- Read operations ---

// Retrieval modes announced on read results so a consumer never mistakes a
// degraded FTS-only answer for the full hybrid-semantic one.
const (
	RetrievalHybridSemantic = "hybrid-semantic"
	RetrievalFTSOnly        = "fts-only"
)

// ftsOnlyReason is the one-line explanation surfaced when semantic retrieval is
// unavailable and the engine degrades to FTS-only.
const ftsOnlyReason = "embeddings unavailable (Ollama unreachable or model absent): serving FTS-only results"

// QueryResult is the outcome of a search. RetrievalMode announces whether the
// answer is hybrid-semantic or a degraded fts-only, RetrievalReason carries the
// one-line cause when degraded, and Reranked records whether the cross-encoder
// reordered the top-k. RerankSource announces where the reranker came from
// (configured, discovered, or off), extending the loud-degradation rule to the
// rerank stage, and RerankReason explains an off source. Mode is the legacy
// human-readable stage string.
type QueryResult struct {
	Query           string      `json:"query"`
	Mode            string      `json:"mode"`
	RetrievalMode   string      `json:"retrieval_mode"`
	RetrievalReason string      `json:"retrieval_reason,omitempty"`
	Reranked        bool        `json:"reranked"`
	RerankSource    string      `json:"rerank_source"`
	RerankReason    string      `json:"rerank_reason,omitempty"`
	Hits            []index.Hit `json:"hits"`
}

// embedQuery embeds the query for retrieval, returning nil when Ollama is
// unavailable or the embed fails. Both Query and QueryMounts call it so a
// mounts-aware search embeds the query exactly once and reuses the vector for
// routing.
func (s *Service) embedQuery(ctx context.Context, query string) []float32 {
	if !s.embed.Available(ctx) {
		return nil
	}
	vecs, err := s.embed.Embed(ctx, []string{query})
	if err != nil || len(vecs) != 1 {
		return nil
	}
	return vecs[0]
}

// Query runs hybrid retrieval (embedding the query when Ollama is available),
// then optional reranking. It announces its retrieval mode: hybrid-semantic when
// the query vector is live, otherwise fts-only with a reason, and records whether
// the reranker actually reordered the results.
func (s *Service) Query(ctx context.Context, query string, limit int) (QueryResult, error) {
	return s.queryWithVec(ctx, query, s.embedQuery(ctx, query), limit)
}

// queryWithVec runs hybrid retrieval with a caller-supplied query vector, so a
// mounts-aware search can reuse the same embedding it computes for routing. A nil
// queryVec degrades to fts-only with an announced reason.
func (s *Service) queryWithVec(ctx context.Context, query string, queryVec []float32, limit int) (QueryResult, error) {
	hits, err := s.store.Hybrid(ctx, query, queryVec, limit)
	if err != nil {
		return QueryResult{}, err
	}

	res := QueryResult{Query: query, RetrievalMode: RetrievalFTSOnly, Mode: "keyword"}
	if queryVec != nil {
		res.RetrievalMode = RetrievalHybridSemantic
		res.Mode = "hybrid"
	} else {
		res.RetrievalReason = ftsOnlyReason
	}

	client, source, reason := s.resolveRerank(ctx)
	res.RerankSource = string(source)
	res.RerankReason = reason
	if client.Enabled() {
		before := hitPaths(hits)
		hits = client.Rerank(ctx, query, hits)
		res.Reranked = !equalStrings(before, hitPaths(hits))
		res.Mode += " + rerank"
	}
	res.Hits = hits
	return res, nil
}

// hitPaths projects a hit slice to its ordered paths, for order comparison.
func hitPaths(hits []index.Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Path
	}
	return out
}

// equalStrings reports whether two string slices are equal in order and content.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// LinkTarget pairs a note's normalized wikilink with the vault-relative path it
// resolves to. Path is empty when the link points at no existing note (broken).
type LinkTarget struct {
	Link string `json:"link"`
	Path string `json:"path"`
}

// Note is a parsed note returned by GetNote.
type Note struct {
	Path        string         `json:"path"`
	Title       string         `json:"title"`
	Tags        []string       `json:"tags"`
	Links       []string       `json:"links"`
	LinkTargets []LinkTarget   `json:"link_targets"`
	Frontmatter map[string]any `json:"frontmatter"`
	Body        string         `json:"body"`
}

// GetNote parses the markdown file at a vault-relative path. The path is cleaned
// and confined to the vault root. Each wikilink in Links is resolved against the
// vault's note set (by normalized name) and reported in LinkTargets, with an
// empty Path for any link that resolves to no note.
func (s *Service) GetNote(_ context.Context, path string) (Note, error) {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+filepath.FromSlash(path))), "/")
	if clean == "" {
		return Note{}, fmt.Errorf("invalid note path: %q", path)
	}
	n, err := vault.Parse(s.Layout.Root, clean)
	if err != nil {
		return Note{}, err
	}
	targets, err := s.resolveLinkCandidates(vault.ExtractWikilinkResolutionCandidates(n.Path, n.Body))
	if err != nil {
		return Note{}, err
	}
	return Note{Path: n.Path, Title: n.Title, Tags: n.Tags, Links: n.Links, LinkTargets: targets, Frontmatter: n.Frontmatter, Body: n.Body}, nil
}

// resolveLinkCandidates maps each wikilink candidate group to the vault-relative
// path of the first candidate that resolves. Unresolved links keep an empty path.
// Order follows the input links.
func (s *Service) resolveLinkCandidates(groups []vault.LinkResolutionCandidates) ([]LinkTarget, error) {
	out := make([]LinkTarget, 0, len(groups))
	if len(groups) == 0 {
		return out, nil
	}
	paths, err := vault.Scan(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]string, len(paths)*2)
	for _, rel := range paths {
		key := vault.GraphKey(rel)
		byName[key] = filepath.ToSlash(rel)
		if alias := vault.GitHubWikiDisplayAlias(key); alias != "" {
			if _, exists := byName[alias]; !exists {
				byName[alias] = filepath.ToSlash(rel)
			}
		}
		legacyKey := vault.NormalizeLink(rel)
		if _, exists := byName[legacyKey]; !exists {
			byName[legacyKey] = filepath.ToSlash(rel)
		}
	}
	for _, group := range groups {
		if group.Primary == "" {
			continue
		}
		target := LinkTarget{Link: group.Primary}
		for _, candidate := range group.Candidates {
			if path := byName[candidate]; path != "" {
				target.Path = path
				break
			}
		}
		out = append(out, target)
	}
	return out, nil
}

// Status is index health. Reranker reports whether a reranker is active (a
// configured or discovered source), and RerankerSource announces which
// (configured, discovered, or off) so an inactive reranker is legible, not
// silent.
type Status struct {
	Root           string `json:"root"`
	Notes          int    `json:"notes"`
	Chunks         int    `json:"chunks"`
	LastIndexed    string `json:"last_indexed_sha"`
	EmbedModel     string `json:"embed_model"`
	Vectors        bool   `json:"vectors"`
	Reranker       bool   `json:"reranker"`
	RerankerSource string `json:"reranker_source"`
}

// Status reports counts, the last indexed commit, and which optional stages are live.
func (s *Service) Status(ctx context.Context) (Status, error) {
	notes, chunks, err := s.store.Count(ctx)
	if err != nil {
		return Status{}, err
	}
	sha, _ := s.store.GetMeta(ctx, "last_indexed_sha")
	model, _ := s.store.GetMeta(ctx, "embed_model")
	if model == "" {
		model = s.embed.Model()
	}
	_, source, _ := s.resolveRerank(ctx)
	return Status{
		Root:           s.Layout.Root,
		Notes:          notes,
		Chunks:         chunks,
		LastIndexed:    sha,
		EmbedModel:     model,
		Vectors:        s.embed.Available(ctx),
		Reranker:       source != rerank.SourceOff,
		RerankerSource: string(source),
	}, nil
}

// graphPageRankTopN bounds how many notes the graph report ranks by centrality.
const graphPageRankTopN = 10

// GraphReport summarizes the derived link graph.
type GraphReport struct {
	Notes    int                   `json:"notes"`
	Links    int                   `json:"links"`
	Orphans  []string              `json:"orphans"`
	Broken   []graph.BrokenLink    `json:"broken"`
	PageRank []graph.PageRankEntry `json:"pagerank"`
}

// Graph derives the link graph from markdown, saves it to the cache, and reports
// orphans, broken links, and the top notes by link-graph centrality (PageRank).
func (s *Service) Graph(_ context.Context) (GraphReport, error) {
	g, err := graph.Build(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return GraphReport{}, err
	}
	if err := g.Save(s.Layout.GraphJSON()); err != nil {
		return GraphReport{}, err
	}
	return GraphReport{
		Notes:    len(g.Nodes),
		Links:    g.EdgeCount(),
		Orphans:  g.Orphans(),
		Broken:   g.BrokenLinks(),
		PageRank: g.TopPageRank(graphPageRankTopN),
	}, nil
}

// --- Cron ---

// CronList returns the configured cron jobs.
func (s *Service) CronList() ([]cron.Job, error) {
	return cron.Load(s.Layout.CronJobs())
}

// CronRun executes a cron job by name, streaming output to w. stardustBin is
// re-execed for command-kind jobs.
func (s *Service) CronRun(ctx context.Context, name, stardustBin string, w io.Writer) error {
	job, err := cron.LoadJob(s.Layout.CronJobs(), name)
	if err != nil {
		return err
	}
	return job.Execute(ctx, stardustBin, s.Layout.Root, w)
}

// RunScheduler runs the cron scheduler loop until ctx is cancelled. It wakes on
// each minute boundary, reloads the cron jobs (skipping malformed ones), and
// fires those whose schedule matches the minute. Manual and event-triggered
// jobs are ignored; overlapping runs of the same job are prevented. stardustBin
// re-execs command-kind jobs; diagnostics and per-job errors go to w.
func (s *Service) RunScheduler(ctx context.Context, stardustBin string, w io.Writer) {
	sched := &cron.Scheduler{}
	timer := time.NewTimer(time.Until(nextMinute(time.Now())))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-timer.C:
			jobs, errs := cron.LoadResilient(s.Layout.CronJobs())
			for _, e := range errs {
				fmt.Fprintf(w, "[stardust cron] load: %v\n", e)
			}
			sched.Tick(ctx, jobs, stardustBin, s.Layout.Root, now, w)
			timer.Reset(time.Until(nextMinute(now)))
		}
	}
}

// nextMinute returns the next whole-minute boundary strictly after t.
func nextMinute(t time.Time) time.Time {
	return t.Truncate(time.Minute).Add(time.Minute)
}
