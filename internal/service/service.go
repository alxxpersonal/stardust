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

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/cron"
	"github.com/alxxpersonal/stardust/internal/embed"
	"github.com/alxxpersonal/stardust/internal/graph"
	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/rerank"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// Service is the open core over one vault: the index, embedder, and reranker.
type Service struct {
	Layout config.Layout
	Config config.Config
	store  *index.Store
	embed  *embed.Client
	rerank *rerank.Client
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
		rerank: rerank.New(cfg.RerankerURL, cfg.RerankerModel),
	}, nil
}

// Close releases the index handle.
func (s *Service) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

// --- Read operations ---

// QueryResult is the outcome of a search.
type QueryResult struct {
	Query string      `json:"query"`
	Mode  string      `json:"mode"`
	Hits  []index.Hit `json:"hits"`
}

// Query runs hybrid retrieval (embedding the query when Ollama is available),
// then optional reranking. Mode records which stages ran.
func (s *Service) Query(ctx context.Context, query string, limit int) (QueryResult, error) {
	var queryVec []float32
	if s.embed.Available(ctx) {
		if vecs, err := s.embed.Embed(ctx, []string{query}); err == nil && len(vecs) == 1 {
			queryVec = vecs[0]
		}
	}
	hits, err := s.store.Hybrid(ctx, query, queryVec, limit)
	if err != nil {
		return QueryResult{}, err
	}
	mode := "keyword"
	if queryVec != nil {
		mode = "hybrid"
	}
	if s.rerank.Enabled() {
		hits = s.rerank.Rerank(ctx, query, hits)
		mode += " + rerank"
	}
	return QueryResult{Query: query, Mode: mode, Hits: hits}, nil
}

// Note is a parsed note returned by GetNote.
type Note struct {
	Path  string   `json:"path"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
	Links []string `json:"links"`
	Body  string   `json:"body"`
}

// GetNote parses the markdown file at a vault-relative path. The path is cleaned
// and confined to the vault root.
func (s *Service) GetNote(_ context.Context, path string) (Note, error) {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+filepath.FromSlash(path))), "/")
	if clean == "" {
		return Note{}, fmt.Errorf("invalid note path: %q", path)
	}
	n, err := vault.Parse(s.Layout.Root, clean)
	if err != nil {
		return Note{}, err
	}
	return Note{Path: n.Path, Title: n.Title, Tags: n.Tags, Links: n.Links, Body: n.Body}, nil
}

// Status is index health.
type Status struct {
	Root        string `json:"root"`
	Notes       int    `json:"notes"`
	Chunks      int    `json:"chunks"`
	LastIndexed string `json:"last_indexed_sha"`
	EmbedModel  string `json:"embed_model"`
	Vectors     bool   `json:"vectors"`
	Reranker    bool   `json:"reranker"`
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
	return Status{
		Root:        s.Layout.Root,
		Notes:       notes,
		Chunks:      chunks,
		LastIndexed: sha,
		EmbedModel:  model,
		Vectors:     s.embed.Available(ctx),
		Reranker:    s.rerank.Enabled(),
	}, nil
}

// GraphReport summarizes the derived link graph.
type GraphReport struct {
	Notes   int                `json:"notes"`
	Links   int                `json:"links"`
	Orphans []string           `json:"orphans"`
	Broken  []graph.BrokenLink `json:"broken"`
}

// Graph derives the link graph from markdown, saves it to the cache, and reports
// orphans and broken links.
func (s *Service) Graph(_ context.Context) (GraphReport, error) {
	g, err := graph.Build(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return GraphReport{}, err
	}
	if err := g.Save(s.Layout.GraphJSON()); err != nil {
		return GraphReport{}, err
	}
	return GraphReport{Notes: len(g.Nodes), Links: g.EdgeCount(), Orphans: g.Orphans(), Broken: g.BrokenLinks()}, nil
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
