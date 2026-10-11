package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type API struct {
	store     *Store
	staticDir string // opsional: folder hasil build frontend
}

type createRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	IntervalSec int    `json:"intervalSec"`
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/monitors", a.list)
	mux.HandleFunc("POST /api/monitors", a.create)
	mux.HandleFunc("GET /api/monitors/{id}", a.get)
	mux.HandleFunc("DELETE /api/monitors/{id}", a.remove)
	mux.HandleFunc("POST /api/monitors/{id}/check", a.checkNow)

	if a.staticDir != "" {
		mux.Handle("/", spaHandler(a.staticDir))
	}
	return chain(mux, recoverer, logger, cors, securityHeaders)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"monitors": len(a.store.List()),
	})
}

func (a *API) list(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.store.List())
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	m, err := a.store.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, &ValidationError{"body JSON tidak valid"})
		return
	}
	m, err := a.store.Add(req.Name, req.URL, req.IntervalSec)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (a *API) remove(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Remove(r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) checkNow(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	m, err := a.store.CheckNow(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// ---- helper respons ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("gagal menulis respons: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var ve *ValidationError
	status := http.StatusInternalServerError
	msg := "terjadi kesalahan pada server"
	switch {
	case errors.As(err, &ve):
		status, msg = http.StatusBadRequest, ve.Msg
	case errors.Is(err, ErrNotFound):
		status, msg = http.StatusNotFound, err.Error()
	case errors.Is(err, ErrLimit):
		status, msg = http.StatusConflict, err.Error()
	default:
		log.Printf("error internal: %v", err)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

// ---- static file + fallback SPA ----

func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint tidak ditemukan"})
			return
		}
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}
