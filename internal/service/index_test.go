package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/rerank"
)

// fakeEmbedder is a deterministic in-memory embedder for tests. It counts how
// many texts it has embedded so a test can assert incremental re-embedding.
type fakeEmbedder struct {
	available bool
	embedded  int
}

func (f *fakeEmbedder) Available(context.Context) bool { return f.available }
func (f *fakeEmbedder) Model() string                  { return "fake" }

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.embedded += len(texts)
	out := make([][]float32, len(texts))
	for i, t := range texts {
		// a tiny deterministic vector so identical text yields identical vectors.
		var sum float32
		for _, r := range t {
			sum += float32(r)
		}
		out[i] = []float32{sum, float32(len(t))}
	}
	return out, nil
}

// newServiceWith builds a Service over a fresh on-disk index with the given
// embedder and reranker endpoint, for internal tests that need to inject fakes.
func newServiceWith(t *testing.T, emb embedder, rerankURL string) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	layout := config.Layout{Root: root}
	require.NoError(t, os.MkdirAll(layout.Cache(), 0o755))
	require.NoError(t, config.Save(layout.Config(), config.Default()))
	st, err := index.Open(context.Background(), layout.DB())
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	svc := &Service{
		Layout: layout,
		Config: config.Default(),
		store:  st,
		embed:  emb,
		rerank: rerank.New(rerankURL, ""),
	}
	return svc, root
}

func TestEmbedPlanReusesUnchangedChunks(t *testing.T) {
	hashes := []string{"h0", "h1", "h2"}
	existing := map[string][]float32{"h0": {1}, "h2": {3}} // h1 is new/changed
	toEmbed, reuse := embedPlan(hashes, existing)
	require.Equal(t, []int{1}, toEmbed)
	require.Len(t, reuse, 2)
	require.Equal(t, []float32{1}, reuse[0])
	require.Equal(t, []float32{3}, reuse[2])
}

func TestIndexReembedsOnlyChangedChunk(t *testing.T) {
	emb := &fakeEmbedder{available: true}
	svc, root := newServiceWith(t, emb, "")
	ctx := context.Background()
	notePath := filepath.Join(root, "n.md")
	write := func(content string) {
		require.NoError(t, os.WriteFile(notePath, []byte(content), 0o644))
	}

	write("---\ntitle: N\n---\n## A\nalpha body\n\n## B\nbeta body\n\n## C\ngamma body\n")
	_, err := svc.Index(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 3, emb.embedded) // three chunks embedded on first index

	// change only section B's body; A and C are byte-identical.
	write("---\ntitle: N\n---\n## A\nalpha body\n\n## B\nBETA CHANGED\n\n## C\ngamma body\n")
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 4, emb.embedded) // only one more chunk re-embedded
}

func TestIndexPrunesRenamedPathsIncrementally(t *testing.T) {
	emb := &fakeEmbedder{available: false}
	svc, root := newServiceWith(t, emb, "")
	ctx := context.Background()

	original := filepath.Join(root, "notes", "old.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(original), 0o755))
	require.NoError(t, os.WriteFile(original, []byte("---\ntitle: Old\n---\n# Old\nbody\n"), 0o644))

	_, err := svc.Index(ctx, "")
	require.NoError(t, err)

	renamed := filepath.Join(root, "notes", "new.md")
	require.NoError(t, os.Rename(original, renamed))

	stats, err := svc.Index(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 1, stats.Deleted)

	catalog, err := svc.store.Catalog(ctx)
	require.NoError(t, err)
	require.NotContains(t, catalog, "notes/old.md")
	require.Contains(t, catalog, "notes/new.md")
}
