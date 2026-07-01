package rpcserver_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/rpcserver"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/rpc"
)

// openRPCDocPath resolves the committed docs/openrpc.json relative to this test
// file via the runtime caller, so the pin is independent of the process working
// directory. This file lives at internal/rpcserver/openrpc_test.go, so the repo
// root is two directories up.
func openRPCDocPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(root, "docs", "openrpc.json")
}

// TestOpenRPCDocMatchesRegistry pins the committed docs/openrpc.json against the
// live registry. The document's method name set MUST equal the set NewRegistry
// exposes, so drift in either direction fails: a method the registry serves but
// the document omits, or a method the document names that the registry no longer
// serves. The registry is the source of truth (its Names() drives rpc.discover via
// rpc.BuildOpenRPC), so the committed file must track it.
func TestOpenRPCDocMatchesRegistry(t *testing.T) {
	raw, err := os.ReadFile(openRPCDocPath(t))
	require.NoError(t, err)

	var doc rpc.OpenRPCDoc
	require.NoError(t, json.Unmarshal(raw, &doc))

	docNames := make([]string, 0, len(doc.Methods))
	for _, m := range doc.Methods {
		docNames = append(docNames, m.Name)
	}
	sort.Strings(docNames)

	svc, err := service.Open(context.Background(), jobsVault(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	regNames := rpcserver.NewRegistry(svc).Names()
	sort.Strings(regNames)

	require.Equal(t, regNames, docNames,
		"docs/openrpc.json method set must equal the live registry method set (drift in either direction fails)")
}
