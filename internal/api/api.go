package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/devakxhay/lighthouse/internal/config"
	"github.com/devakxhay/lighthouse/internal/db"
	"github.com/devakxhay/lighthouse/internal/nginx"
	"github.com/devakxhay/lighthouse/internal/process"
	"github.com/devakxhay/lighthouse/internal/ssl"
	"github.com/devakxhay/lighthouse/internal/templates"
	"github.com/devakxhay/lighthouse/models"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	DB      *db.DB
	Cfg     *config.Config
	Nginx   *nginx.Manager
	Process *process.Manager
	SSL     *ssl.Generator
}

// ---- helpers ----

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

// ---- App Handlers ----

// GET /api/apps
func (h *Handler) ListApps(w http.ResponseWriter, r *http.Request) {
	apps, err := h.DB.ListApps()
	if err != nil {
		writeErr(w, 500, "failed to list apps")
		return
	}

	// Enrich with live systemd status
	type AppWithStatus struct {
		models.App
		LiveStatus string `json:"live_status"`
	}

	enriched := make([]AppWithStatus, 0, len(apps))
	for _, a := range apps {
		st, _ := h.Process.Status(a.Name)
		liveStatus := "stopped"
		if st != nil && st.Sub == "running" {
			liveStatus = "running"
		} else if st != nil && st.Active == "failed" {
			liveStatus = "failed"
		}
		enriched = append(enriched, AppWithStatus{App: a, LiveStatus: liveStatus})
	}

	writeJSON(w, 200, enriched)
}

// POST /api/apps
func (h *Handler) CreateApp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Type   string `json:"type"` // spring-boot | nextjs | go
		Domain string `json:"domain"`
		Port   int    `json:"port"`
		GitURL string `json:"git_url"`
		AppDir string `json:"app_dir"`
		EntryPoint string `json:"entry_point"`
	}

	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "invalid request body")
		return
	}

	if req.Name == "" || req.Type == "" || req.Domain == "" || req.GitURL == "" {
		writeErr(w, 400, "name, type, domain, and git_url are required")
		return
	}

	if req.Port == 0 {
		freePort, err := findFreePort(h.DB)
		if err != nil {
			writeErr(w, 500, fmt.Sprintf("failed to allocate automatic port: %v", err))
			return
		}
		req.Port = freePort
	}

	app := &models.App{
		Name:       req.Name,
		Type:       models.AppType(req.Type),
		Domain:     req.Domain,
		Port:       req.Port,
		GitURL:     req.GitURL,
		AppDir:     req.AppDir,
		EntryPoint: req.EntryPoint,
	}
	
	if err := h.DB.CreateApp(app); err != nil {
		writeErr(w, 500, fmt.Sprintf("create app: %s", err))
		return
	}

	h.DB.Log(models.AuditLog{
		AppName: app.Name,
		Action:  models.ActionDeploy,
		Status:  "SUCCESS",
		Details: fmt.Sprintf("App registered: type=%s domain=%s port=%d", app.Type, app.Domain, app.Port),
	})

	writeJSON(w, 201, app)
}

// DELETE /api/apps/{name}
func (h *Handler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	h.Process.Stop(name)
	h.Process.RemoveUnit(name)
	h.Nginx.RemoveConfig(name)
	h.DB.DeleteApp(name)

	h.DB.Log(models.AuditLog{
		AppName: name,
		Action:  models.ActionStop,
		Status:  "SUCCESS",
		Details: "App deleted and all resources removed",
	})

	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// ---- Deploy Handler ----

// POST /api/apps/{name}/deploy
// Full deploy: pull/build → generate cert → write nginx → create systemd unit → restart
func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	app, err := h.DB.GetApp(name)
	if err != nil || app == nil {
		writeErr(w, 404, "app not found")
		return
	}

	steps := []string{}
	fail := func(step string, err error) {
		h.DB.Log(models.AuditLog{
			AppName: name,
			Action:  models.ActionDeploy,
			Status:  "FAILED",
			Details: fmt.Sprintf("Failed at step [%s]: %s", step, err),
		})
		writeErr(w, 500, fmt.Sprintf("[%s] %s", step, err))
	}

	// 0. Pull code and build
	if err := h.pullAndBuild(app); err != nil {
		fail("BUILD", err)
		return
	}
	steps = append(steps, "code pulled and built")

	if err := h.DB.UpdateAppPaths(app.Name, app.AppDir, app.BinaryPath); err != nil {
		fail("SAVE_PATHS", err)
		return
	}

	// 1. Generate SSL cert
	certPaths, err := h.SSL.Generate(app.Domain)
	if err != nil {
		fail("CERT_GEN", err)
		return
	}
	steps = append(steps, "cert generated")

	// 2. Save cert to DB
	cert := &models.Cert{
		AppID:     app.ID,
		Domain:    app.Domain,
		IssuedAt:  certPaths.IssuedAt,
		ExpiresAt: certPaths.ExpiresAt,
		CertPath:  certPaths.CertPath,
		KeyPath:   certPaths.KeyPath,
	}
	h.DB.SaveCert(cert)
	h.DB.Log(models.AuditLog{
		AppName: name,
		Action:  models.ActionCertGen,
		Status:  "SUCCESS",
		Details: fmt.Sprintf("Cert for %s — expires %s", app.Domain, certPaths.ExpiresAt.Format("2006-01-02")),
	})

	// 3. Write env file (empty if none set)
	envFile := filepath.Join(h.Cfg.Dirs.Envs, name+".env")
	os.MkdirAll(h.Cfg.Dirs.Envs, 0755)
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		os.WriteFile(envFile, []byte("# Lighthouse env file for "+name+"\n"), 0600)
	}

	// 4. Write nginx config
	nginxContent, err := templates.RenderNginxConf(templates.AppData{
		Name:     app.Name,
		Domain:   app.Domain,
		Port:     app.Port,
		CertPath: certPaths.CertPath,
		KeyPath:  certPaths.KeyPath,
	})
	if err != nil {
		fail("NGINX_TEMPLATE", err)
		return
	}
	if err := h.Nginx.WriteConfig(name, nginxContent); err != nil {
		fail("NGINX_WRITE", err)
		return
	}
	steps = append(steps, "nginx configured")
	h.DB.Log(models.AuditLog{
		AppName: name,
		Action:  models.ActionNginxReload,
		Status:  "SUCCESS",
		Details: "Nginx config written and reloaded",
	})

	// 5. Write systemd unit
	unitContent, err := templates.RenderSystemdUnit(string(app.Type), templates.AppData{
		Name:       app.Name,
		BinaryPath: app.BinaryPath,
		AppDir:     app.AppDir,
		Port:       app.Port,
		EnvFile:    envFile,
	})
	if err != nil {
		fail("SYSTEMD_TEMPLATE", err)
		return
	}
	if err := h.Process.WriteUnit(name, unitContent); err != nil {
		fail("SYSTEMD_WRITE", err)
		return
	}
	h.Process.Enable(name)
	steps = append(steps, "systemd unit created")

	// 6. Restart the service
	if err := h.Process.Restart(name); err != nil {
		fail("SERVICE_START", err)
		return
	}
	steps = append(steps, "service restarted")

	h.DB.UpdateAppStatus(name, models.StatusRunning)
	h.DB.Log(models.AuditLog{
		AppName: name,
		Action:  models.ActionDeploy,
		Status:  "SUCCESS",
		Details: fmt.Sprintf("Deploy complete. Steps: %v", steps),
	})

	writeJSON(w, 200, map[string]any{
		"status": "deployed",
		"steps":  steps,
		"domain": "https://" + app.Domain,
	})
}

// ---- Process Control ----

// POST /api/apps/{name}/start
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := h.Process.Start(name)
	status := "SUCCESS"
	if err != nil {
		status = "FAILED"
	}
	h.DB.Log(models.AuditLog{AppName: name, Action: models.ActionStart, Status: status})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	h.DB.UpdateAppStatus(name, models.StatusRunning)
	writeJSON(w, 200, map[string]string{"status": "started"})
}

// POST /api/apps/{name}/stop
func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := h.Process.Stop(name)
	status := "SUCCESS"
	if err != nil {
		status = "FAILED"
	}
	h.DB.Log(models.AuditLog{AppName: name, Action: models.ActionStop, Status: status})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	h.DB.UpdateAppStatus(name, models.StatusStopped)
	writeJSON(w, 200, map[string]string{"status": "stopped"})
}

// POST /api/apps/{name}/restart
func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := h.Process.Restart(name)
	status := "SUCCESS"
	if err != nil {
		status = "FAILED"
	}
	h.DB.Log(models.AuditLog{AppName: name, Action: models.ActionRestart, Status: status})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "restarted"})
}

// GET /api/apps/{name}/logs?lines=100
func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	lines := 100
	if l := r.URL.Query().Get("lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			lines = n
		}
	}
	logs, err := h.Process.Logs(name, lines)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"logs": logs})
}

// ---- Audit ----

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

// ---- Nginx Rollback ----

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

// findFreePort locates an available TCP port in the range 30000-45000.
func findFreePort(database *db.DB) (int, error) {
	apps, err := database.ListApps()
	if err != nil {
		return 0, err
	}
	usedPorts := make(map[int]bool)
	for _, a := range apps {
		usedPorts[a.Port] = true
	}

	for port := 30000; port <= 45000; port++ {
		if usedPorts[port] {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free ports available in range 30000-45000")
}

// GET /api/apps/next-port
func (h *Handler) NextPort(w http.ResponseWriter, r *http.Request) {
	port, err := findFreePort(h.DB)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]int{"port": port})
}

// POST /api/apps/{name}/entry-point
func (h *Handler) UpdateEntryPoint(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req struct {
		EntryPoint string `json:"entry_point"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "invalid request body")
		return
	}

	if err := h.DB.UpdateAppEntryPoint(name, req.EntryPoint); err != nil {
		writeErr(w, 500, fmt.Sprintf("failed to update entry point: %v", err))
		return
	}

	h.DB.Log(models.AuditLog{
		AppName: name,
		Action:  models.ActionEnvChange,
		Status:  "SUCCESS",
		Details: fmt.Sprintf("Updated Go entry point to: %s", req.EntryPoint),
	})

	writeJSON(w, 200, map[string]string{"status": "updated", "entry_point": req.EntryPoint})
}
