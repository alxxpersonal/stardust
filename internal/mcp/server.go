// Package mcp serves the Stardust core over the Model Context Protocol (stdio),
// so agents (Claude Code and other MCP clients) can search the vault as a tool.
// It mirrors the modelcontextprotocol/go-sdk server pattern. stdout carries
// JSON-RPC; all logging must go to stderr.
package mcp

import (
	"context"
	"errors"
	"io"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alxxpersonal/stardust/internal/service"
)

const version = "0.1.0"

// Serve runs the Stardust MCP server over stdio until the client disconnects or
// ctx is cancelled. A clean disconnect (EOF) or cancellation is not an error.
func Serve(ctx context.Context, svc *service.Service) error {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "stardust", Version: version}, nil)
	registerTools(server, svc)
	if err := server.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	return nil
}
