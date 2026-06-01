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
)

// newServeCmd runs a Stardust server. v1 serves the HTTP/JSON API.
func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Stardust HTTP/JSON API server",
		Long:  "Serves the HTTP/JSON API over the same core the CLI uses (see docs/openapi.yaml).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAPI(cmd, addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7777", "listen address")
	return cmd
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
