package fusion

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRRFCombinesAcrossLists(t *testing.T) {
	// "b" appears near the top of both lists, so it should win.
	a := []string{"a", "b", "c"}
	b := []string{"b", "d", "a"}
	scores := RRF(60, a, b)

	keys := make([]string, 0, len(scores))
	for k := range scores {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return scores[keys[i]] > scores[keys[j]] })

	require.Equal(t, "b", keys[0])
	require.Greater(t, scores["a"], scores["c"]) // a is in both lists, c in one
}

func TestRRFDefaultK(t *testing.T) {
	s0 := RRF(0, []string{"x"})
	s60 := RRF(60, []string{"x"})
	require.Equal(t, s60["x"], s0["x"]) // k<=0 falls back to DefaultK
}

func TestRRFEmpty(t *testing.T) {
	require.Empty(t, RRF(60))
}
