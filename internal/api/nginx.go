package api

import (
	"fmt"
	"net/http"

	"github.com/devakxhay/lighthouse/models"
	"github.com/go-chi/chi/v5"
)

// GET /api/apps/{name}/nginx/history
func (h *Handler) NginxHistory(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	history, err := h.Nginx.History(name)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, history)
}

// POST /api/apps/{name}/nginx/rollback
func (h *Handler) NginxRollback(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req struct {
		Hash string `json:"hash"`
	}
	if err := decode(r, &req); err != nil || req.Hash == "" {
		writeErr(w, 400, "hash is required")
		return
	}

	err := h.Nginx.Rollback(name, req.Hash)
	status := "SUCCESS"
	if err != nil {
		status = "FAILED"
	}
	h.DB.Log(models.AuditLog{
		AppName: name,
		Action:  models.ActionRollback,
		Status:  status,
		Details: fmt.Sprintf("Nginx config rolled back to %s", req.Hash),
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "rolled back", "hash": req.Hash})
}
