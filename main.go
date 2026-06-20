package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/devakxhay/lighthouse/internal/api"
	"github.com/devakxhay/lighthouse/internal/config"
	"github.com/devakxhay/lighthouse/internal/db"
	"github.com/devakxhay/lighthouse/internal/nginx"
	"github.com/devakxhay/lighthouse/internal/process"
	"github.com/devakxhay/lighthouse/internal/ssl"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed ui/*
var uiFS embed.FS

func main() {
	// Config
	cfgPath := os.Getenv("LIGHTHOUSE_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Ensure dirs exist
	for _, dir := range []string{cfg.Dirs.Certs, cfg.Dirs.Envs, cfg.Dirs.Data} {
		os.MkdirAll(dir, 0755)
	}

	// DB
	database, err := db.New(cfg.Dirs.Data + "/lighthouse.db")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	// Nginx manager — init git repo
	ngx := &nginx.Manager{
		SitesAvailable: cfg.Nginx.SitesAvailable,
		SitesEnabled:   cfg.Nginx.SitesEnabled,
	}
	if err := ngx.Init(); err != nil {
		log.Printf("warn: nginx git init: %v", err)
	}

	// Handler
	h := &api.Handler{
		DB:    database,
		Cfg:   cfg,
		Nginx: ngx,
		Process: &process.Manager{
			UnitsDir: cfg.Dirs.Units,
		},
		SSL: &ssl.Generator{
			CACertPath: cfg.CA.CertPath,
			CAKeyPath:  cfg.CA.KeyPath,
			CertsDir:   cfg.Dirs.Certs,
		},
	}

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)

	// UI
	tmpl, err := template.ParseFS(uiFS, "ui/index.html", "ui/components/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, nil); err != nil {
			log.Printf("render index template: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	})
	r.Get("/style.css", func(w http.ResponseWriter, req *http.Request) {
		data, _ := uiFS.ReadFile("ui/style.css")
		w.Header().Set("Content-Type", "text/css")
		w.Write(data)
	})
	r.Get("/app.js", func(w http.ResponseWriter, req *http.Request) {
		data, _ := uiFS.ReadFile("ui/app.js")
		w.Header().Set("Content-Type", "application/javascript")
		w.Write(data)
	})

	// API
	r.Route("/api", func(r chi.Router) {
		r.Route("/apps", func(r chi.Router) {
			r.Get("/", h.ListApps)
			r.Post("/", h.CreateApp)

			r.Route("/{name}", func(r chi.Router) {
				r.Delete("/", h.DeleteApp)
				r.Post("/deploy", h.Deploy)
				r.Post("/start", h.Start)
				r.Post("/stop", h.Stop)
				r.Post("/restart", h.Restart)
				r.Get("/logs", h.GetLogs)
				r.Get("/audit", h.GetAudit)
				r.Get("/nginx/history", h.NginxHistory)
				r.Post("/nginx/rollback", h.NginxRollback)
			})
		})
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("🔦 Lighthouse running at http://%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
