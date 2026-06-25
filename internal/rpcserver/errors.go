package rpcserver

import (
	"errors"

	"github.com/creachadair/jrpc2"

	"github.com/alxxpersonal/stardust/internal/collections"
)

// Domain error codes are positive integers, placed outside the reserved JSON-RPC
// server band of -32000..-32099 (ADR 0006). A domain error means the request was
// well formed but its data is wrong; it is the caller's fault, not the server's.
// Infrastructure failures are left as plain errors so jrpc2 maps them into the
// reserved band via jrpc2.ErrorCode.
const (
	// CodeValidation marks a record-frontmatter schema violation.
	CodeValidation jrpc2.Code = 1000
)

// domainError classifies err and, when it is a recognized domain failure, returns
// a jrpc2.Error carrying the matching positive code. Any other error (or nil) is
// returned unchanged so the transport assigns it a reserved-band code. Wrap a
// handler's terminal error through this so domain and infrastructure failures
// land in their separate code bands.
func domainError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, collections.ErrValidation) {
		return jrpc2.Errorf(CodeValidation, "%s", err.Error())
	}
	return err
}
