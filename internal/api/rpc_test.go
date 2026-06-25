package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/api"
	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

// newRPCServer stands an httptest server over a freshly indexed vault, the same
// fixture the REST tests use, so the jrpc2 bridge runs against the real service
// core.
func newRPCServer(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.WriteFile(filepath.Join(root, "note.md"),
		[]byte("---\ntitle: Test Note\n---\n# Test Note\nhello world"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)

	srv := httptest.NewServer(api.New(svc))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestAPI_RPCStatus(t *testing.T) {
	url := newRPCServer(t) + "/rpc"

	envelope := `{"jsonrpc":"2.0","id":1,"method":"status"}`
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(envelope)) //nolint:noctx
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var env struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Notes int `json:"notes"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.Equal(t, "2.0", env.JSONRPC)
	require.Equal(t, 1, env.ID)
	require.Nil(t, env.Error)
	require.Equal(t, 1, env.Result.Notes)
}

func TestAPI_RPCMethodNotAllowed(t *testing.T) {
	url := newRPCServer(t) + "/rpc"
	resp, err := http.Get(url) //nolint:gosec,noctx
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}
