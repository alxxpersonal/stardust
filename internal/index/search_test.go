package index

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVecRoundtrip(t *testing.T) {
	v := []float32{0.1, -0.5, 1.0, 0, 3.14}
	require.Equal(t, v, decodeVec(encodeVec(v)))
}

func TestCosine(t *testing.T) {
	require.InDelta(t, 1.0, cosine([]float32{1, 0}, []float32{1, 0}), 1e-6)
	require.InDelta(t, 0.0, cosine([]float32{1, 0}, []float32{0, 1}), 1e-6)
	require.InDelta(t, -1.0, cosine([]float32{1, 0}, []float32{-1, 0}), 1e-6)
	require.Equal(t, 0.0, cosine([]float32{1, 0}, []float32{0, 0}))    // zero vector
	require.Equal(t, 0.0, cosine([]float32{1, 0}, []float32{1, 2, 3})) // length mismatch
}

func TestFtsQuery(t *testing.T) {
	require.Equal(t, `"hello" OR "world"`, ftsQuery("hello, world!"))
	require.Equal(t, `"go"`, ftsQuery("go"))
	require.Equal(t, "", ftsQuery("  !!! ??? "))
}
