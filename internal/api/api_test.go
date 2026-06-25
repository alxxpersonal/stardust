package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAPI_Health verifies the liveness probe survives the REST retirement.
func TestAPI_Health(t *testing.T) {
	srv := newRPCServer(t)
	resp, err := http.Get(srv + "/healthz") //nolint:gosec,noctx
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestAPI_RESTRetired confirms the retired REST routes are gone: every former
// route 404s now that /rpc is the sole programmatic surface (ADR 0004). Only
// /healthz and /rpc remain mounted.
func TestAPI_RESTRetired(t *testing.T) {
	srv := newRPCServer(t)

	retired := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/query?q=hello"},
		{http.MethodGet, "/note?path=note.md"},
		{http.MethodGet, "/status"},
		{http.MethodGet, "/graph"},
		{http.MethodGet, "/bundle?task=x"},
		{http.MethodGet, "/check"},
		{http.MethodGet, "/digest"},
		{http.MethodGet, "/cron"},
		{http.MethodGet, "/mounts"},
		{http.MethodGet, "/collections"},
		{http.MethodGet, "/collection?name=jobs"},
		{http.MethodGet, "/records?collection=jobs"},
		{http.MethodGet, "/record?path=note.md"},
		{http.MethodPost, "/index"},
		{http.MethodPost, "/rebuild"},
		{http.MethodPost, "/archive"},
		{http.MethodPost, "/cron/run?job=x"},
		{http.MethodPost, "/records"},
		{http.MethodPatch, "/record?path=note.md"},
		{http.MethodDelete, "/record?path=note.md"},
	}

	for _, rt := range retired {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req, err := http.NewRequest(rt.method, srv+rt.path, nil) //nolint:noctx
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	}
}
