package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/api"
	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	mountsDir := config.Layout{Root: root}.Mounts()
	require.NoError(t, os.MkdirAll(filepath.Join(mountsDir, "gmail"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mountsDir, "gmail", "config.toml"),
		[]byte("command = \"gmail-mcp\"\ntool = \"search\"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "note.md"),
		[]byte("---\ntitle: Test Note\n---\n# Test Note\nhello world content about gardening, see [[other]]"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "other.md"),
		[]byte("---\ntitle: Other\n---\n# Other\nlinked from note, see [[note]]"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	srv := httptest.NewServer(api.New(svc))
	t.Cleanup(srv.Close)
	return srv
}

func getJSON(t *testing.T, url string, v any) {
	t.Helper()
	resp, err := http.Get(url) //nolint:gosec,noctx
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(v))
}

func TestAPI_Health(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/healthz") //nolint:gosec,noctx
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAPI_StatusAndQuery(t *testing.T) {
	srv := newTestServer(t)

	var st struct {
		Notes int `json:"notes"`
	}
	getJSON(t, srv.URL+"/status", &st)
	require.Equal(t, 2, st.Notes)

	var qr struct {
		Hits []struct {
			Path string `json:"path"`
		} `json:"hits"`
	}
	getJSON(t, srv.URL+"/query?q=gardening", &qr)
	require.NotEmpty(t, qr.Hits)
	require.Equal(t, "note.md", qr.Hits[0].Path)
}

func TestAPI_Note(t *testing.T) {
	srv := newTestServer(t)
	var n struct {
		Title       string `json:"title"`
		Body        string `json:"body"`
		LinkTargets []struct {
			Link string `json:"link"`
			Path string `json:"path"`
		} `json:"link_targets"`
	}
	getJSON(t, srv.URL+"/note?path=note.md", &n)
	require.Equal(t, "Test Note", n.Title)
	require.Contains(t, n.Body, "gardening")
	require.Len(t, n.LinkTargets, 1)
	require.Equal(t, "other", n.LinkTargets[0].Link)
	require.Equal(t, "other.md", n.LinkTargets[0].Path)
}

func TestAPI_Mounts(t *testing.T) {
	srv := newTestServer(t)
	var ms []struct {
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Target string `json:"target"`
		Tool   string `json:"tool"`
	}
	getJSON(t, srv.URL+"/mounts", &ms)
	require.Len(t, ms, 1)
	require.Equal(t, "gmail", ms[0].Name)
	require.Equal(t, "mcp", ms[0].Kind)
	require.Equal(t, "gmail-mcp", ms[0].Target)
	require.Equal(t, "search", ms[0].Tool)
}

func TestAPI_GraphPageRank(t *testing.T) {
	srv := newTestServer(t)
	var rep struct {
		Notes    int `json:"notes"`
		PageRank []struct {
			Path  string  `json:"path"`
			Title string  `json:"title"`
			Score float64 `json:"score"`
		} `json:"pagerank"`
	}
	getJSON(t, srv.URL+"/graph", &rep)
	require.Equal(t, 2, rep.Notes)
	require.NotEmpty(t, rep.PageRank)
	require.NotEmpty(t, rep.PageRank[0].Path)
	require.Greater(t, rep.PageRank[0].Score, 0.0)
}

func TestAPI_QueryMissingParam(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/query") //nolint:gosec,noctx
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
