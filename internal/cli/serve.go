package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/api"
	"github.com/alxxpersonal/stardust/internal/mcp"
)

// newServeCmd runs a Stardust server: the HTTP/JSON API, or the MCP server over
// stdio with --mcp.
func newServeCmd() *cobra.Command {
	var addr string
	var mcpMode bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a Stardust server (HTTP/JSON API, or MCP over stdio with --mcp)",
		Long:  "Serves the same core the CLI uses. Default: HTTP/JSON API (see docs/openapi.yaml).\nWith --mcp: an MCP server over stdio for agents (Claude Code).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if mcpMode {
				return runMCP(cmd)
			}
			return runAPI(cmd, addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7777", "HTTP API listen address")
	cmd.Flags().BoolVar(&mcpMode, "mcp", false, "serve the MCP server over stdio instead of HTTP")
	return cmd
}

func runMCP(cmd *cobra.Command) error {
	svc, err := openService(cmd.Context())
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()
	return mcp.Serve(cmd.Context(), svc)
}

func runAPI(cmd *cobra.Command, addr string) error {
	svc, err := openService(cmd.Context())
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()

	srv := &http.Server{Addr: addr, Handler: api.New(svc), ReadHeaderTimeout: 10 * time.Second}

	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "stardust api listening on http://%s\n", addr)
		errCh <- srv.ListenAndServe()
	}()

	sigCtx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Drive scheduled cron jobs for as long as the server runs. The scheduler
	// stops when sigCtx is cancelled (SIGINT/SIGTERM). os.Executable re-execs
	// command-kind jobs back through stardust.
	if exe, exeErr := os.Executable(); exeErr == nil {
		go svc.RunScheduler(sigCtx, exe, os.Stderr)
	} else {
		fmt.Fprintf(os.Stderr, "cron scheduler not started: %v\n", exeErr)
	}

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("api server: %w", err)
		}
		return nil
	case <-sigCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	}
}
