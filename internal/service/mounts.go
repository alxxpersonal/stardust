package service

import (
	"context"
	"sort"
	"strconv"

	"github.com/alxxpersonal/stardust/internal/fusion"
	"github.com/alxxpersonal/stardust/internal/mounts"
)

// FusedHit is a result from the local vault or a mount, after RRF fusion.
type FusedHit struct {
	Source  string  `json:"source"` // "vault" or a mount name
	Title   string  `json:"title"`
	Ref     string  `json:"ref"` // note path or external id
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"` // fused RRF score
}

// QueryMounts fans the query out to the local index and every configured mount,
// then fuses the rankings with RRF. A mount that errors is skipped, so the local
// results and the other mounts still return.
func (s *Service) QueryMounts(ctx context.Context, query string, limit int) ([]FusedHit, error) {
	ms, err := mounts.Load(s.Layout.Mounts())
	if err != nil {
		return nil, err
	}

	registry := map[string]FusedHit{}
	var lists [][]string

	local, err := s.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	localKeys := make([]string, 0, len(local.Hits))
	for _, h := range local.Hits {
		key := "vault:" + h.Path
		localKeys = append(localKeys, key)
		registry[key] = FusedHit{Source: "vault", Title: h.Title, Ref: h.Path, Snippet: h.Snippet}
	}
	lists = append(lists, localKeys)

	for _, m := range ms {
		hits, mErr := m.Search(ctx, query, limit)
		if mErr != nil {
			continue // graceful: a failing mount does not fail the whole query
		}
		keys := make([]string, 0, len(hits))
		for i, h := range hits {
			key := m.Name + ":" + h.Ref + ":" + strconv.Itoa(i)
			keys = append(keys, key)
			registry[key] = FusedHit{Source: h.Source, Title: h.Title, Ref: h.Ref, Snippet: h.Snippet}
		}
		lists = append(lists, keys)
	}

	scores := fusion.RRF(fusion.DefaultK, lists...)
	out := make([]FusedHit, 0, len(scores))
	for key, sc := range scores {
		fh := registry[key]
		fh.Score = sc
		out = append(out, fh)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// MountNames returns the names of configured mounts.
func (s *Service) MountNames() ([]string, error) {
	ms, err := mounts.Load(s.Layout.Mounts())
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ms))
	for i, m := range ms {
		names[i] = m.Name
	}
	return names, nil
}
