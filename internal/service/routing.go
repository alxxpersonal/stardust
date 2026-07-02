package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"unicode"

	"github.com/alxxpersonal/stardust/internal/mounts"
)

// routeCosineThreshold is the minimum cosine similarity between the query vector
// and a mount's embedded description for that mount to be a semantic routing
// candidate. It is set deliberately low: false-exclude is the costly error (a
// silent recall loss), so routing over-includes rather than risk dropping a
// relevant mount. See ADR 0042.
const routeCosineThreshold = 0.35

// routeMount is the routing view of one configured mount: its name and optional
// self-description, plus the embedded description vector when semantic routing is
// live. The caller precomputes everything so routePlanFor launches no subprocess
// and embeds nothing.
type routeMount struct {
	name        string
	description string
	keywords    []string
	descVec     []float32 // embedded description; nil when absent or unavailable
}

func (m routeMount) hasMetadata() bool {
	return strings.TrimSpace(m.description) != "" || len(m.keywords) > 0
}

// routePlan is the decision routePlanFor returns: which mounts to fan out to,
// which were pruned, the routing mode, and a one-line human reason. It carries no
// hits; QueryMounts executes the plan.
type routePlan struct {
	search  []string
	skipped []string
	mode    string
	reason  string
}

// routePlanFor decides which mounts a query fans out to, before any mount
// subprocess launches. It enforces ADR 0042's invariant and conservative
// fallback: prune an external mount only when confident it is irrelevant, never
// prune a metadata-less mount, and search everything on any ambiguity. scope is
// an explicit --mounts=a,b list (nil for none); queryVec is the reused query
// embedding (nil in fts-only or when the query embed failed, in which case
// routing matches lexically).
func routePlanFor(ms []routeMount, scope []string, query string, queryVec []float32) routePlan {
	all := make([]string, len(ms))
	for i, m := range ms {
		all[i] = m.name
	}

	// Single-mount and no-mount workspaces never route: byte-identical to today.
	if len(ms) <= 1 {
		return routePlan{search: all, mode: RoutingAll}
	}

	// Explicit scope wins, highest confidence. An explicit list and a mount name
	// mentioned in the query text both scope directly and bypass the
	// metadata-less union: the user named exactly what they want.
	if explicit := explicitScope(ms, scope, query); explicit != nil {
		return gate(explicit, all, RoutingRouted, "matched explicit scope")
	}

	// Routing needs at least one self-describing mount to act on. With none there
	// is nothing to route by, so search all and stay quiet (byte-identical to a
	// pre-metadata workspace: no routing line renders for mode all).
	described := false
	for _, m := range ms {
		if m.hasMetadata() {
			described = true
			break
		}
	}
	if !described {
		return routePlan{search: all, mode: RoutingAll}
	}

	// Soft routing over the metadata-bearing mounts. A metadata-less mount can
	// never be confidently excluded, so it is always a candidate. In semantic
	// mode a described mount with no usable vector also cannot be scored, so it
	// stays in too.
	semantic := queryVec != nil
	var matched []string
	for _, m := range ms {
		switch {
		case !m.hasMetadata():
			matched = append(matched, m.name)
		case semantic:
			if m.descVec == nil || cosineSim(queryVec, m.descVec) >= routeCosineThreshold {
				matched = append(matched, m.name)
			}
		case lexicalMatch(query, m):
			matched = append(matched, m.name)
		}
	}

	return gate(matched, all, RoutingRouted, "matched mount metadata")
}

// gate applies the conservative fallback (ADR 0042): a plan routes only when it
// is a strict, non-empty subset of the mount set. An empty subset or a subset
// covering every mount both mean routing pruned nothing safely, so search all
// with mode fallback. The returned search and skipped lists follow the canonical
// mount order (all is already sorted by mounts.Load).
func gate(subset, all []string, routedMode, routedReason string) routePlan {
	in := make(map[string]bool, len(subset))
	for _, n := range subset {
		in[n] = true
	}
	search := make([]string, 0, len(subset))
	skipped := make([]string, 0, len(all))
	for _, n := range all {
		if in[n] {
			search = append(search, n)
		} else {
			skipped = append(skipped, n)
		}
	}

	if len(search) == 0 {
		return routePlan{search: all, mode: RoutingFallback, reason: "no mount matched; searching all"}
	}
	if len(search) == len(all) {
		return routePlan{search: all, mode: RoutingFallback, reason: "routing pruned nothing; searching all"}
	}
	return routePlan{search: search, skipped: skipped, mode: routedMode, reason: routedReason}
}

// explicitScope returns the mounts named by an explicit --mounts=a,b list or by a
// mount name appearing as a token in the query text, mapped back to canonical
// mount names. A given-but-all-unknown list returns a non-nil empty slice so the
// caller's gate turns it into a safe fallback (never scope to zero on a typo). It
// returns nil only when there is no explicit signal at all.
func explicitScope(ms []routeMount, scope []string, query string) []string {
	canon := make(map[string]string, len(ms))
	for _, m := range ms {
		canon[strings.ToLower(m.name)] = m.name
	}
	if len(scope) > 0 {
		out := []string{}
		for _, s := range scope {
			if name, ok := canon[strings.ToLower(strings.TrimSpace(s))]; ok {
				out = append(out, name)
			}
		}
		return out // non-nil: an explicit list was given, even if it names nothing real
	}
	tokens := tokenize(query)
	var out []string
	for _, m := range ms {
		if tokens[strings.ToLower(m.name)] {
			out = append(out, m.name)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// lexicalMatch reports whether the query shares at least one case-folded token
// with the mount's name, keywords, or description. It is the fts-only routing
// signal, used when no query vector is available.
func lexicalMatch(query string, m routeMount) bool {
	q := tokenize(query)
	if len(q) == 0 {
		return false
	}
	fields := append([]string{m.name, m.description}, m.keywords...)
	for _, f := range fields {
		for t := range tokenize(f) {
			if q[t] {
				return true
			}
		}
	}
	return false
}

// tokenize splits text into a set of case-folded alphanumeric tokens.
func tokenize(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		out[f] = true
	}
	return out
}

// cosineSim returns the cosine similarity of two equal-length vectors, or 0 when
// lengths differ or either is zero.
func cosineSim(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// routeMounts assembles the routing view of the configured mounts. It attaches an
// embedded description vector only when a query vector is live (semantic mode),
// caching each vector in the index meta table keyed by a hash of the embed model
// and the description text so an edit to either invalidates it. A cache miss is
// embedded in one batched call, far cheaper than the subprocess launches routing
// avoids.
func (s *Service) routeMounts(ctx context.Context, ms []mounts.Mount, queryVec []float32) []routeMount {
	out := make([]routeMount, len(ms))
	for i, m := range ms {
		out[i] = routeMount{name: m.Name, description: m.Cfg.Description, keywords: m.Cfg.Keywords}
	}
	if queryVec == nil {
		return out // fts-only: lexical routing needs no vectors
	}

	model := s.embed.Model()
	var missIdx []int
	var missText []string
	for i := range out {
		desc := strings.TrimSpace(out[i].description)
		if desc == "" {
			continue
		}
		if cached, err := s.store.GetMeta(ctx, routeVecKey(model, desc)); err == nil && cached != "" {
			if v := decodeRouteVec(cached); v != nil {
				out[i].descVec = v
				continue
			}
		}
		missIdx = append(missIdx, i)
		missText = append(missText, desc)
	}
	if len(missText) == 0 {
		return out
	}
	vecs, err := s.embed.Embed(ctx, missText)
	if err != nil || len(vecs) != len(missText) {
		return out // embed failed: described mounts stay unvectored -> never excluded
	}
	for j, idx := range missIdx {
		out[idx].descVec = vecs[j]
		if enc := encodeRouteVec(vecs[j]); enc != "" {
			_ = s.store.SetMeta(ctx, routeVecKey(model, strings.TrimSpace(out[idx].description)), enc)
		}
	}
	return out
}

func routeVecKey(model, description string) string {
	sum := sha256.Sum256([]byte(model + "\n" + description))
	return "route_vec:" + hex.EncodeToString(sum[:])
}

func encodeRouteVec(v []float32) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func decodeRouteVec(s string) []float32 {
	var v []float32
	if err := json.Unmarshal([]byte(s), &v); err != nil || len(v) == 0 {
		return nil
	}
	return v
}
