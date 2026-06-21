package api

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/devakxhay/lighthouse/internal/config"
	"github.com/devakxhay/lighthouse/internal/db"
	"github.com/devakxhay/lighthouse/internal/nginx"
	"github.com/devakxhay/lighthouse/internal/process"
	"github.com/devakxhay/lighthouse/internal/ssl"
)

type Handler struct {
	DB      *db.DB
	Cfg     *config.Config
	Nginx   *nginx.Manager
	Process *process.Manager
	SSL     *ssl.Generator
	log     *slog.Logger
}

func NewHandler(db *db.DB, cfg *config.Config, ngx *nginx.Manager, proc *process.Manager, sslGen *ssl.Generator, logger *slog.Logger) *Handler {
	return &Handler{
		DB:      db,
		Cfg:     cfg,
		Nginx:   ngx,
		Process: proc,
		SSL:     sslGen,
		log:     logger.With(slog.String("component", "api")),
	}
}

// ---- helpers ----

func getIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}


