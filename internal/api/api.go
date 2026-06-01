// Package api serves the Stardust HTTP/JSON API over the core Service. Every
// route is a thin wrapper around a Service method, so the API has the same
// capabilities as the CLI and the MCP server.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/alxxpersonal/stardust/internal/service"
)

// Handler serves the API over a single open Service.
type Handler struct {
	svc *service.Service
	mu  sync.Mutex // serialises write operations (index/rebuild) against the single-conn store
	mux *http.ServeMux
}

// New builds an API handler over svc.
func New(svc *service.Service) *Handler {
	h := &Handler{svc: svc, mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /healthz", h.health)
	h.mux.HandleFunc("GET /query", h.query)
	h.mux.HandleFunc("GET /note", h.note)
	h.mux.HandleFunc("GET /status", h.status)
	h.mux.HandleFunc("GET /graph", h.graph)
	h.mux.HandleFunc("GET /bundle", h.bundle)
	h.mux.HandleFunc("GET /cron", h.cronList)
	h.mux.HandleFunc("POST /index", h.index)
	h.mux.HandleFunc("POST /rebuild", h.rebuild)
	h.mux.HandleFunc("POST /archive", h.archive)
	h.mux.HandleFunc("POST /cron/run", h.cronRun)
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.mux.ServeHTTP(w, r) }

// --- Handlers ---

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) query(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeErr(w, http.StatusBadRequest, errors.New("missing required query parameter: q"))
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if r.URL.Query().Get("mounts") == "true" {
		fused, err := h.svc.QueryMounts(r.Context(), q, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"query": q, "hits": fused})
		return
	}
	res, err := h.svc.Query(r.Context(), q, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) note(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeErr(w, http.StatusBadRequest, errors.New("missing required query parameter: path"))
		return
	}
	n, err := h.svc.GetNote(r.Context(), p)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Status(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *Handler) graph(w http.ResponseWriter, r *http.Request) {
	rep, err := h.svc.Graph(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (h *Handler) bundle(w http.ResponseWriter, r *http.Request) {
	task := r.URL.Query().Get("task")
	if task == "" {
		writeErr(w, http.StatusBadRequest, errors.New("missing required query parameter: task"))
		return
	}
	budget := 4000
	if b := r.URL.Query().Get("budget"); b != "" {
		if n, err := strconv.Atoi(b); err == nil && n > 0 {
			budget = n
		}
	}
	res, err := h.svc.Bundle(r.Context(), task, budget)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) cronList(w http.ResponseWriter, _ *http.Request) {
	jobs, err := h.svc.CronList()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stats, err := h.svc.Index(r.Context(), r.URL.Query().Get("since"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) rebuild(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stats, err := h.svc.Rebuild(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	path, err := h.svc.Archive(r.Context(), r.URL.Query().Get("dest"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (h *Handler) cronRun(w http.ResponseWriter, r *http.Request) {
	job := r.URL.Query().Get("job")
	if job == "" {
		writeErr(w, http.StatusBadRequest, errors.New("missing required query parameter: job"))
		return
	}
	exe, _ := os.Executable()
	var buf bytes.Buffer
	if err := h.svc.CronRun(r.Context(), job, exe, &buf); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error(), "output": buf.String()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": buf.String()})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
