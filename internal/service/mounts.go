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

// Routing modes announced on a mounts-aware search, mirroring RetrievalMode.
// RoutingAll means the fan-out searched every mount without routing (single or
// no mount, or no mount carries routing metadata); RoutingRouted means routing
// pruned to a confident, strict subset; RoutingFallback means routing engaged
// but backed off to searching all mounts on low confidence.
const (
	RoutingAll      = "all"
	RoutingRouted   = "routed"
	RoutingFallback = "fallback"
)

// MountQueryResult is the outcome of a mounts-aware search. It mirrors
// QueryResult's visibility contract: RoutingMode announces whether the fan-out
// searched all mounts, routed to a confident subset, or fell back to all on low
// confidence; RoutingReason carries the one-line cause; MountsSearched and
// MountsSkipped name the mounts on each side of the decision; RetrievalMode and
// RetrievalReason are inherited from the local query so a consumer sees both the
// routing and the retrieval story at once.
type MountQueryResult struct {
	Query           string     `json:"query"`
	Hits            []FusedHit `json:"hits"`
	RoutingMode     string     `json:"routing_mode"`
	RoutingReason   string     `json:"routing_reason,omitempty"`
	MountsSearched  []string   `json:"mounts_searched"`
	MountsSkipped   []string   `json:"mounts_skipped,omitempty"`
	RetrievalMode   string     `json:"retrieval_mode"`
	RetrievalReason string     `json:"retrieval_reason,omitempty"`
}

// QueryMounts fans the query out to the local vault and the configured mounts a
// query is about, then fuses the rankings with RRF. It computes a routing plan
// before launching any connector (see ADR 0042): scope is an explicit
// --mounts=a,b list (nil for all), routing only prunes an external mount it is
// confident is irrelevant, and it falls back to searching every mount on any
// ambiguity. The local vault is always searched. A mount that errors is skipped,
// so the local results and the other mounts still return.
func (s *Service) QueryMounts(ctx context.Context, query string, limit int, scope []string) (MountQueryResult, error) {
	ms, err := mounts.Load(s.Layout.Mounts())
	if err != nil {
		return MountQueryResult{}, err
	}

	// Embed the query once and reuse the vector for both local retrieval and
	// semantic routing, so a mounts-aware search never double-embeds.
	queryVec := s.embedQuery(ctx, query)

	local, err := s.queryWithVec(ctx, query, queryVec, limit)
	if err != nil {
		return MountQueryResult{}, err
	}

	// Plan the fan-out before any subprocess launches.
	plan := routePlanFor(s.routeMounts(ctx, ms, queryVec), scope, query, queryVec)
	planned := make(map[string]bool, len(plan.search))
	for _, n := range plan.search {
		planned[n] = true
	}

	result := MountQueryResult{
		Query:           query,
		RoutingMode:     plan.mode,
		RoutingReason:   plan.reason,
		MountsSearched:  plan.search,
		MountsSkipped:   plan.skipped,
		RetrievalMode:   local.RetrievalMode,
		RetrievalReason: local.RetrievalReason,
	}

	registry := map[string]FusedHit{}
	var lists [][]string

	localKeys := make([]string, 0, len(local.Hits))
	for _, h := range local.Hits {
		key := "vault:" + h.Path
		localKeys = append(localKeys, key)
		registry[key] = FusedHit{Source: "vault", Title: h.Title, Ref: h.Path, Snippet: h.Snippet}
	}
	lists = append(lists, localKeys)

	for _, m := range ms {
		if !planned[m.Name] {
			continue // routed out: do not pay its subprocess-launch tax
		}
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
	result.Hits = out
	return result, nil
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

// MountInfo describes one configured mount as read from its config.toml. Every
// mount is an MCP-server connector, so Kind is always "mcp" and Target is the
// executable launched to reach the source.
type MountInfo struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`           // connector kind; "mcp" for all mounts today
	Target string   `json:"target"`         // the executable launched for this mount
	Args   []string `json:"args,omitempty"` // arguments passed to the executable
	Tool   string   `json:"tool"`           // the mount's search tool name
}

// Mounts returns the configured mounts read from .stardust/mounts, sorted by
// name (mounts.Load already sorts). It is the read-only inventory behind the
// /mounts surface; it never launches a connector.
func (s *Service) Mounts() ([]MountInfo, error) {
	ms, err := mounts.Load(s.Layout.Mounts())
	if err != nil {
		return nil, err
	}
	out := make([]MountInfo, len(ms))
	for i, m := range ms {
		out[i] = MountInfo{
			Name:   m.Name,
			Kind:   "mcp",
			Target: m.Cfg.Command,
			Args:   m.Cfg.Args,
			Tool:   m.Cfg.Tool,
		}
	}
	return out, nil
}
