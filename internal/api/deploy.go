package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/devakxhay/lighthouse/internal/templates"
	"github.com/devakxhay/lighthouse/models"
	"github.com/go-chi/chi/v5"
)

// POST /api/apps/{name}/deploy
// Full deploy: pull/build → generate cert → write nginx → create systemd unit → restart
func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	name := chi.URLParam(r, "name")

	h.log.Info(fmt.Sprintf("POST /api/apps/%s/deploy", name), "ip", getIP(r))

	app, err := h.DB.GetApp(name)
	if err != nil || app == nil {
		h.log.Warn("app not found", "name", name)
		writeErr(w, 404, "app not found")
		return
	}

	if app.Status == models.StatusBuilding {
		h.log.Warn("deploy blocked: already building", "app", name)
		writeErr(w, 400, "Application is currently building/deploying. Please wait.")
		return
	}

	steps := []string{}
	fail := func(step string, err error) {
		h.log.Error("deploy failed", "app", name, "step", step, "error", err.Error())
		h.DB.UpdateAppStatus(name, models.StatusFailed)
		h.DB.Log(models.AuditLog{
			AppName: name,
			Action:  models.ActionDeploy,
			Status:  "FAILED",
			Details: fmt.Sprintf("Failed at step [%s]: %s", step, err),
		})
		writeErr(w, 500, fmt.Sprintf("[%s] %s", step, err))
	}

	// Update status to building
	if err := h.DB.UpdateAppStatus(name, models.StatusBuilding); err != nil {
		fail("START_BUILD", err)
		return
	}

	// Pre-flight check: required runtime must be configured
	var requiredRuntime = map[models.AppType]string{
		models.AppTypeSpringBoot: "java",
		models.AppTypeNextJS:     "npm",
		models.AppTypeGo:         "go",
	}

	if reqRuntime, ok := requiredRuntime[app.Type]; ok {
		rt, err := h.DB.GetRuntime(reqRuntime)
		if err != nil {
			fail("CHECK_RUNTIME", fmt.Errorf("failed to retrieve runtime details: %w", err))
			return
		}
		if rt == nil || rt.BinPath == "" {
			h.log.Warn("runtime missing for deploy", "app", name, "required", reqRuntime)
			writeErr(w, 400, fmt.Sprintf("%s not configured. Go to Settings to set the path.", reqRuntime))
			return
		}
	}

	// 0. Pull code and build
	if err := h.pullAndBuild(app); err != nil {
		fail("BUILD", err)
		return
	}
	steps = append(steps, "build")

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
	steps = append(steps, "cert")

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
	steps = append(steps, "nginx")
	h.DB.Log(models.AuditLog{
		AppName: name,
		Action:  models.ActionNginxReload,
		Status:  "SUCCESS",
		Details: "Nginx config written and reloaded",
	})

	// Fetch all runtime paths from DB to populate AppData
	var javaBin, npmBin, goBin, nodeBin string
	if rt, _ := h.DB.GetRuntime("java"); rt != nil {
		javaBin = rt.BinPath
	}
	if rt, _ := h.DB.GetRuntime("npm"); rt != nil {
		npmBin = rt.BinPath
	}
	if rt, _ := h.DB.GetRuntime("go"); rt != nil {
		goBin = rt.BinPath
	}
	if rt, _ := h.DB.GetRuntime("node"); rt != nil {
		nodeBin = rt.BinPath
	}

	pathEnv := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	if nodeBin != "" {
		pathEnv = filepath.Dir(nodeBin) + ":" + pathEnv
	}
	if npmBin != "" {
		pathEnv = filepath.Dir(npmBin) + ":" + pathEnv
	}
	if goBin != "" {
		pathEnv = filepath.Dir(goBin) + ":" + pathEnv
	}
	if javaBin != "" {
		pathEnv = filepath.Dir(javaBin) + ":" + pathEnv
	}

	// 5. Write systemd unit
	unitContent, err := templates.RenderSystemdUnit(string(app.Type), templates.AppData{
		Name:       app.Name,
		BinaryPath: app.BinaryPath,
		AppDir:     app.AppDir,
		Port:       app.Port,
		EnvFile:    envFile,
		JavaBin:    javaBin,
		NpmBin:     npmBin,
		GoBin:      goBin,
		PathEnv:    pathEnv,
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
	steps = append(steps, "systemd")

	// 6. Restart the service
	if err := h.Process.Restart(name); err != nil {
		fail("SERVICE_START", err)
		return
	}
	steps = append(steps, "start")

	h.DB.UpdateAppStatus(name, models.StatusRunning)
	h.DB.Log(models.AuditLog{
		AppName: name,
		Action:  models.ActionDeploy,
		Status:  "SUCCESS",
		Details: fmt.Sprintf("Deploy complete. Steps: %v", steps),
	})

	elapsed := time.Since(startTime)
	h.log.Info("deploy complete", "app", name, "steps", steps, "duration", fmt.Sprintf("%.1fs", elapsed.Seconds()))

	writeJSON(w, 200, map[string]any{
		"status": "deployed",
		"steps":  steps,
		"domain": "https://" + app.Domain,
	})
}
