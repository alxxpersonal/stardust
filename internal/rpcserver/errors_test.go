package rpcserver_test

import (
	"context"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/rpc"
)

// TestErrorBandInfrastructure pins the infrastructure error band. A failure that
// is not a domain error (here, reading a record that does not exist) MUST carry a
// JSON-RPC code in the reserved -32000..-32099 server band (ADR 0006). jrpc2 maps
// a plain returned error to that band by default; this test guards against a
// handler accidentally re-coding an infra failure into the domain space.
func TestErrorBandInfrastructure(t *testing.T) {
	ctx := context.Background()
	loc := localFromRegistry(t, jobsVault(t))
	defer func() { _ = loc.Close() }()

	var rec rpc.Record
	err := loc.Client.CallResult(ctx, "record/get", rpc.RecordParams{Path: "jobs/does-not-exist.md"}, &rec)
	require.Error(t, err)

	code := int(jrpc2.ErrorCode(err))
	require.GreaterOrEqual(t, code, -32099, "infrastructure error code must be within the reserved server band")
	require.LessOrEqual(t, code, -32000, "infrastructure error code must be within the reserved server band")
}

// TestErrorBandDomain pins the domain error band. A schema validation failure is
// a domain error and MUST carry a positive JSON-RPC code, outside the reserved
// -32000..-32099 server band (ADR 0006). This separates "the request was well
// formed but the data is wrong" from "the server or its environment failed".
func TestErrorBandDomain(t *testing.T) {
	ctx := context.Background()
	loc := localFromRegistry(t, jobsVault(t))
	defer func() { _ = loc.Close() }()

	// The jobs schema requires "company"; omitting it is a domain validation
	// failure, not an infrastructure failure.
	var rec rpc.Record
	err := loc.Client.CallResult(ctx, "record/create", rpc.CreateRecordParams{
		Collection: "jobs",
		Fields:     map[string]any{"status": "open"},
		Body:       "no company",
	}, &rec)
	require.Error(t, err)

	code := int(jrpc2.ErrorCode(err))
	require.Positive(t, code, "domain error code must be a positive integer outside the reserved server band")
}
