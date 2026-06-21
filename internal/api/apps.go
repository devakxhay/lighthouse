package api

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/devakxhay/lighthouse/models"
	"github.com/go-chi/chi/v5"
)

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
		Name       string `json:"name"`
		Type       string `json:"type"` // spring-boot | nextjs | go
		Domain     string `json:"domain"`
		Port       int    `json:"port"`
		GitURL     string `json:"git_url"`
		AppDir     string `json:"app_dir"`
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

	h.log.Info("app registered", "name", app.Name, "type", string(app.Type), "domain", app.Domain, "port", app.Port)

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

	h.log.Info("app deleted", "name", name)

	h.DB.Log(models.AuditLog{
		AppName: name,
		Action:  models.ActionStop,
		Status:  "SUCCESS",
		Details: "App deleted and all resources removed",
	})

	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// POST /api/apps/{name}/start
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, err := h.DB.GetApp(name)
	if err != nil || app == nil {
		h.log.Warn("app not found", "name", name)
		writeErr(w, 404, "app not found")
		return
	}

	if app.Status == models.StatusBuilding {
		writeErr(w, 400, "Application is currently building/deploying. Please wait.")
		return
	}
	if app.Status == models.StatusPending {
		writeErr(w, 400, "Application has not been deployed yet. Please deploy it first.")
		return
	}
	if app.Status == models.StatusRunning {
		writeErr(w, 400, "Application is already running.")
		return
	}

	err = h.Process.Start(name)
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
	app, err := h.DB.GetApp(name)
	if err != nil || app == nil {
		h.log.Warn("app not found", "name", name)
		writeErr(w, 404, "app not found")
		return
	}

	if app.Status == models.StatusBuilding {
		writeErr(w, 400, "Application is currently building/deploying. Please wait.")
		return
	}
	if app.Status == models.StatusPending {
		writeErr(w, 400, "Application has not been deployed yet. Please deploy it first.")
		return
	}
	if app.Status == models.StatusStopped {
		writeErr(w, 400, "Application is already stopped.")
		return
	}

	err = h.Process.Stop(name)
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
	app, err := h.DB.GetApp(name)
	if err != nil || app == nil {
		h.log.Warn("app not found", "name", name)
		writeErr(w, 404, "app not found")
		return
	}

	if app.Status == models.StatusBuilding {
		writeErr(w, 400, "Application is currently building/deploying. Please wait.")
		return
	}
	if app.Status == models.StatusPending {
		writeErr(w, 400, "Application has not been deployed yet. Please deploy it first.")
		return
	}

	err = h.Process.Restart(name)
	status := "SUCCESS"
	if err != nil {
		status = "FAILED"
	}
	h.DB.Log(models.AuditLog{AppName: name, Action: models.ActionRestart, Status: status})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	h.DB.UpdateAppStatus(name, models.StatusRunning)
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

// findFreePort locates an available TCP port in the range 30000-45000.
func findFreePort(database interface{ ListApps() ([]models.App, error) }) (int, error) {
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
