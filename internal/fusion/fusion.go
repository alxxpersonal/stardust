// Package fusion implements Reciprocal Rank Fusion, the rank-based merge that
// combines ranked lists from different sources (BM25, vectors, separate mounts)
// without any score calibration. It is the one primitive that lets Stardust fuse
// the local index with N heterogeneous mounted sources.
package fusion

// DefaultK is the standard RRF constant. Larger k flattens the contribution of
// top ranks; 60 is the widely used default.
const DefaultK = 60

// RRF fuses several best-first ranked lists of keys into a combined score per
// key. A key's score is the sum over lists of 1/(k + rank + 1), where rank is
// its 0-based position in that list. Higher score is better. Keys absent from a
// list simply do not contribute from it.
func RRF(k int, lists ...[]string) map[string]float64 {
	if k <= 0 {
		k = DefaultK
	}
	scores := make(map[string]float64)
	for _, list := range lists {
		for rank, key := range list {
			scores[key] += 1.0 / float64(k+rank+1)
		}
	}
	return scores
}
