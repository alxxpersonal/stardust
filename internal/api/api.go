// Package api serves the Stardust HTTP surface: a liveness probe at GET
// /healthz and the typed jrpc2 registry mounted as a jhttp bridge at POST /rpc.
// The REST routes that once mirrored every Service method were retired once the
// JSON-RPC contract reached structural parity and both consumers migrated to
// /rpc (ADR 0004); /rpc is now the sole programmatic surface.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/creachadair/jrpc2/jhttp"

	"github.com/alxxpersonal/stardust/internal/rpcserver"
	"github.com/alxxpersonal/stardust/internal/service"
)

// Handler serves the API over a single open Service.
type Handler struct {
	mux *http.ServeMux
}

// New builds an API handler over svc. It mounts the typed jrpc2 registry as a
// jhttp bridge at POST /rpc and a liveness probe at GET /healthz; every Service
// operation is reachable through the bridge.
func New(svc *service.Service) *Handler {
	h := &Handler{mux: http.NewServeMux()}
	bridge := jhttp.NewBridge(rpcserver.NewRegistry(svc), &jhttp.BridgeOptions{Server: rpcserver.ServerOptions()})
	h.mux.Handle("/rpc", bridge)
	h.mux.HandleFunc("GET /healthz", h.health)
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.mux.ServeHTTP(w, r) }

// --- Handlers ---

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
