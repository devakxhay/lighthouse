package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devakxhay/lighthouse/internal/api"
	"github.com/devakxhay/lighthouse/internal/config"
	"github.com/devakxhay/lighthouse/internal/db"
	"github.com/devakxhay/lighthouse/internal/dns"
	"github.com/devakxhay/lighthouse/internal/logger"
	"github.com/devakxhay/lighthouse/internal/nginx"
	"github.com/devakxhay/lighthouse/internal/process"
	"github.com/devakxhay/lighthouse/internal/runtime"
	"github.com/devakxhay/lighthouse/internal/ssl"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed ui/*
var uiFS embed.FS

func main() {
	// Dev mode flag
	var devMode bool
	flag.BoolVar(&devMode, "dev", false, "Enable dev mode")
	flag.Parse()

	// Initialize global logger
	log := logger.New(devMode)

	log.Info("startup: Lighthouse starting", "version", "x", "port", 9000, "dev", devMode)

	// Config
	cfgPath := os.Getenv("LIGHTHOUSE_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.dev.yml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Error("startup: config load failed — fatal", "error", err.Error())
		os.Exit(1)
	}
	log.Info("startup: config loaded", "path", cfgPath)

	// Ensure dirs exist
	for _, dir := range []string{cfg.Dirs.Certs, cfg.Dirs.Envs, cfg.Dirs.Data} {
		os.MkdirAll(dir, 0755)
	}

	// DB
	database, err := db.New(cfg.Dirs.Data+"/lighthouse.db", log)
	if err != nil {
		log.Error("startup: db init failed — fatal", "error", err.Error())
		os.Exit(1)
	}
	log.Info("startup: db initialized", "path", cfg.Dirs.Data+"/lighthouse.db")

	// Runtime detection on startup
	detector := runtime.NewDetector(database, cfg, log)
	rts, err := detector.Detect()
	if err != nil {
		log.Error("startup: runtime detection failed", "error", err.Error())
	} else {
		var runtimeAttrs []any
		for _, rt := range rts {
			val := rt.BinPath
			if val == "" {
				val = "missing"
			}
			runtimeAttrs = append(runtimeAttrs, rt.Name, val)
		}
		log.Info("startup: runtime detection complete", runtimeAttrs...)
	}

	// Nginx manager — init git repo
	ngx := nginx.NewManager(cfg.Nginx.SitesAvailable, cfg.Nginx.SitesEnabled, devMode, log)
	if err := ngx.Init(); err != nil {
		log.Error("startup: nginx git init failed", "error", err.Error())
	} else {
		log.Info("startup: nginx git repo initialized", "path", cfg.Nginx.SitesAvailable)
	}

	pm := process.NewManager(cfg.Dirs.Units, devMode, log)
	sslGen := ssl.NewGenerator(cfg.CA.Dir, cfg.Dirs.Certs, log)
	dnsMgr := dns.NewManager(cfg.DNS.DnsmasqConf, devMode, log)

	// Handler
	h := api.NewHandler(database, cfg, ngx, pm, sslGen, dnsMgr, log)

	// Router
	r := chi.NewRouter()
	r.Use(HTTPLogger(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)

	// UI
	tmpl, err := template.ParseFS(uiFS, "ui/index.html", "ui/components/*.html")
	if err != nil {
		log.Error("parse templates failed", "error", err.Error())
		os.Exit(1)
	}

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, nil); err != nil {
			log.Error("render index template failed", "error", err.Error())
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
	r.Get("/favicon.png", func(w http.ResponseWriter, req *http.Request) {
		data, _ := uiFS.ReadFile("ui/favicon.png")
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	})

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/certs", h.GetCerts)
		r.Get("/certs/ca", h.GetCACert)
		r.Get("/certs/ca/status", h.GetCACertStatus)

		r.Route("/runtimes", func(r chi.Router) {
			r.Get("/", h.GetRuntimes)
			r.Post("/detect", h.DetectRuntimes)
			r.Put("/{name}", h.OverrideRuntime)
		})

		r.Route("/apps", func(r chi.Router) {
			r.Get("/", h.ListApps)
			r.Post("/", h.CreateApp)
			r.Get("/next-port", h.NextPort)

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
				r.Post("/entry-point", h.UpdateEntryPoint)
			})
		})
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info("startup: server listening", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Error("server failed", "error", err.Error())
		os.Exit(1)
	}
}

// HTTPLogger Chi request logging middleware using slog
func HTTPLogger(log *slog.Logger) func(http.Handler) http.Handler {
	logger := log.With(slog.String("component", "http"))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			defer func() {
				duration := time.Since(t1)
				status := ww.Status()
				ip := getIP(r)

				lvl := slog.LevelInfo
				if status >= 400 && status < 500 {
					lvl = slog.LevelWarn
				} else if status >= 500 {
					lvl = slog.LevelError
				}

				path := r.URL.Path
				if r.URL.RawQuery != "" {
					path = path + "?" + r.URL.RawQuery
				}

				logger.Log(r.Context(), lvl, fmt.Sprintf("%-4s %s", r.Method, path),
					slog.Int("status", status),
					slog.String("duration", fmt.Sprintf("%.3fs", duration.Seconds())),
					slog.String("ip", ip),
				)
			}()
			next.ServeHTTP(ww, r)
		})
	}
}

func getIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

