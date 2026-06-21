package api

import (
	"fmt"
	"net/http"
)

// GET /api/certs
func (h *Handler) GetCerts(w http.ResponseWriter, r *http.Request) {
	certs, err := h.SSL.ListIssued()
	if err != nil {
		h.log.Error("failed to list issued certificates", "error", err.Error())
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("list certs: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, certs)
}
