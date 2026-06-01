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
	require.NoError(t, os.WriteFile(filepath.Join(root, "note.md"),
		[]byte("---\ntitle: Test Note\n---\n# Test Note\nhello world content about gardening"), 0o644))

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
	require.Equal(t, 1, st.Notes)

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
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	getJSON(t, srv.URL+"/note?path=note.md", &n)
	require.Equal(t, "Test Note", n.Title)
	require.Contains(t, n.Body, "gardening")
}

func TestAPI_QueryMissingParam(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/query") //nolint:gosec,noctx
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
