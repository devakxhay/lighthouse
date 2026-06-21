package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// GET /api/apps/{name}/audit?limit=50
func (h *Handler) GetAudit(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	logs, err := h.DB.GetAuditLogs(name, limit)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, logs)
}
