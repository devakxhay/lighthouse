package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

// GET /api/certs/ca
func (h *Handler) GetCACert(w http.ResponseWriter, r *http.Request) {
	caPath := filepath.Join(h.SSL.CADir, "ca.crt")
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		h.log.Error("CA certificate not found", "path", caPath)
		writeErr(w, http.StatusNotFound, "CA certificate not found")
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=ca.crt")
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	http.ServeFile(w, r, caPath)
}

// GET /api/certs/ca/status
func (h *Handler) GetCACertStatus(w http.ResponseWriter, r *http.Request) {
	caPath := filepath.Join(h.SSL.CADir, "ca.crt")
	exists := false
	if _, err := os.Stat(caPath); err == nil {
		exists = true
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"exists": exists,
	})
}


