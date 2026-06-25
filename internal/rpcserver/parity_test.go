package rpcserver_test

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/api"
	"github.com/alxxpersonal/stardust/internal/rpcserver"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/rpc"
)

// parityService opens a service over a fresh temp "jobs" vault and indexes it,
// returning the service and its root. Each transport gets its own vault so the
// mutating round-trip runs independently on each, then the two surfaces are
// compared. Record results are vault-relative so they are byte-identical across
// vaults; only the status root differs, which the test normalizes against root.
// The service is closed via t.Cleanup.
func parityService(t *testing.T) (*service.Service, string) {
	t.Helper()
	root := jobsVault(t)
	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	_, err = svc.Index(context.Background(), "")
	require.NoError(t, err)
	return svc, root
}

// stdioClient stands a registry over svc behind a real newline-framed stdio
// channel pair (os.Pipe both directions), the same framing a JSON-RPC stdio loop
// uses, and returns a jrpc2 client driving it. The server and pipes are torn down
// via t.Cleanup; svc is owned by the caller.
func stdioClient(t *testing.T, svc *service.Service) *jrpc2.Client {
	t.Helper()

	// Two unidirectional pipes wired into a duplex client/server pair, each end
	// framed with channel.Line (newline-delimited JSON), the canonical stdio
	// framing.
	cr, sw, err := os.Pipe()
	require.NoError(t, err)
	sr, cw, err := os.Pipe()
	require.NoError(t, err)
	serverCh := channel.Line(sr, sw)
	clientCh := channel.Line(cr, cw)

	srv := jrpc2.NewServer(rpcserver.NewRegistry(svc), nil).Start(serverCh)
	client := jrpc2.NewClient(clientCh, nil)
	t.Cleanup(func() {
		_ = client.Close()
		srv.Stop()
		_ = srv.Wait()
	})
	return client
}

// httpClient stands a registry over svc behind the jhttp bridge (POST /rpc on an
// httptest server) and returns a jrpc2 client over a jhttp channel, so both
// transports yield a *jrpc2.Response whose raw result bytes can be compared. svc
// is owned by the caller.
func httpClient(t *testing.T, svc *service.Service) *jrpc2.Client {
	t.Helper()

	srv := httptest.NewServer(api.New(svc))
	t.Cleanup(srv.Close)

	ch := jhttp.NewChannel(srv.URL+"/rpc", nil)
	client := jrpc2.NewClient(ch, nil)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// callRaw invokes method with params on client and returns the raw JSON result
// bytes, so a byte-level comparison across transports is possible.
func callRaw(t *testing.T, client *jrpc2.Client, method string, params any) string {
	t.Helper()
	rsp, err := client.Call(context.Background(), method, params)
	require.NoError(t, err)
	require.Nil(t, rsp.Error())
	return rsp.ResultString()
}

// TestTransportParity pins stdio and HTTP transport parity. The same registry,
// reached over the jrpc2 stdio channel and the jhttp bridge, MUST return
// byte-identical JSON results for status and for a record round-trip (create,
// get, list, patch, delete). A surface that diverges in field set, ordering, or
// encoding fails this test (plan Task E4, spec Verification). Each transport runs
// the round-trip on its own vault; record results are vault-relative so they are
// byte-identical, and status differs only in the absolute root, which the test
// normalizes against each vault's known root.
func TestTransportParity(t *testing.T) {
	stdioSvc, stdioRoot := parityService(t)
	httpSvc, httpRoot := parityService(t)
	stdio := stdioClient(t, stdioSvc)
	http := httpClient(t, httpSvc)

	// status: identical apart from the absolute vault root. Normalize each side's
	// own root to a placeholder, then the remaining bytes must match exactly.
	stdioStatus := strings.Replace(callRaw(t, stdio, "status", nil), stdioRoot, "<root>", 1)
	httpStatus := strings.Replace(callRaw(t, http, "status", nil), httpRoot, "<root>", 1)
	require.Equal(t, stdioStatus, httpStatus,
		"status result must be byte-identical across stdio and http (root normalized)")

	// record round-trip: every result is vault-relative, so each step's raw JSON
	// must match byte-for-byte across the two transports.
	create := rpc.CreateRecordParams{
		Collection: "jobs",
		Fields:     map[string]any{"company": "Acme", "status": "open", "score": float64(8)},
		Body:       "first body",
	}
	require.Equal(t, callRaw(t, stdio, "record/create", create), callRaw(t, http, "record/create", create),
		"record/create result must be byte-identical across stdio and http")

	get := rpc.RecordParams{Path: "jobs/acme.md"}
	require.Equal(t, callRaw(t, stdio, "record/get", get), callRaw(t, http, "record/get", get),
		"record/get result must be byte-identical across stdio and http")

	list := rpc.ListRecordsParams{Collection: "jobs"}
	require.Equal(t, callRaw(t, stdio, "record/list", list), callRaw(t, http, "record/list", list),
		"record/list result must be byte-identical across stdio and http")

	body := "patched body"
	patch := rpc.PatchRecordParams{Path: "jobs/acme.md", Fields: map[string]any{"status": "closed"}, Body: &body}
	require.Equal(t, callRaw(t, stdio, "record/patch", patch), callRaw(t, http, "record/patch", patch),
		"record/patch result must be byte-identical across stdio and http")

	del := rpc.RecordParams{Path: "jobs/acme.md"}
	require.Equal(t, callRaw(t, stdio, "record/delete", del), callRaw(t, http, "record/delete", del),
		"record/delete result must be byte-identical across stdio and http")
}
