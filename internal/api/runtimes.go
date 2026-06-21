package api

import (
	"fmt"
	"net/http"

	"github.com/devakxhay/lighthouse/internal/runtime"
	"github.com/go-chi/chi/v5"
)

type RuntimeResponse struct {
	Name       string `json:"name"`
	BinPath    string `json:"bin_path"`
	Found      bool   `json:"found"`
	Overridden bool   `json:"overridden"`
}

// GET /api/runtimes
func (h *Handler) GetRuntimes(w http.ResponseWriter, r *http.Request) {
	runtimes, err := h.DB.ListRuntimes()
	if err != nil {
		writeErr(w, 500, "failed to list runtimes")
		return
	}

	resp := make([]RuntimeResponse, len(runtimes))
	for i, rt := range runtimes {
		resp[i] = RuntimeResponse{
			Name:       rt.Name,
			BinPath:    rt.BinPath,
			Found:      rt.BinPath != "",
			Overridden: rt.Overridden,
		}
	}
	writeJSON(w, 200, resp)
}

// POST /api/runtimes/detect
func (h *Handler) DetectRuntimes(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "true"
	if force {
		if err := h.DB.ResetRuntimeOverrides(); err != nil {
			writeErr(w, 500, fmt.Sprintf("failed to reset overrides: %v", err))
			return
		}
	}

	detector := runtime.NewDetector(h.DB, h.Cfg, h.log)
	if _, err := detector.Detect(); err != nil {
		writeErr(w, 500, fmt.Sprintf("runtime detection failed: %v", err))
		return
	}

	runtimes, err := h.DB.ListRuntimes()
	if err != nil {
		writeErr(w, 500, "failed to reload runtimes")
		return
	}

	resp := make([]RuntimeResponse, len(runtimes))
	for i, rt := range runtimes {
		resp[i] = RuntimeResponse{
			Name:       rt.Name,
			BinPath:    rt.BinPath,
			Found:      rt.BinPath != "",
			Overridden: rt.Overridden,
		}
	}
	writeJSON(w, 200, resp)
}

// PUT /api/runtimes/{name}
func (h *Handler) OverrideRuntime(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req struct {
		BinPath string `json:"bin_path"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "invalid request body")
		return
	}

	if err := h.DB.SetRuntimeOverride(name, req.BinPath); err != nil {
		writeErr(w, 500, fmt.Sprintf("failed to set override: %v", err))
		return
	}

	writeJSON(w, 200, map[string]string{"status": "updated"})
}
