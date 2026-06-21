package nginx

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Manager struct {
	SitesAvailable string
	SitesEnabled   string
	DevMode        bool
	log            *slog.Logger
}

func NewManager(sitesAvailable, sitesEnabled string, devMode bool, logger *slog.Logger) *Manager {
	return &Manager{
		SitesAvailable: sitesAvailable,
		SitesEnabled:   sitesEnabled,
		DevMode:        devMode,
		log:            logger.With(slog.String("component", "nginx")),
	}
}

// WriteConfig writes a new nginx config, tests it, and commits on success.
// On failure, it reverts and returns an error.
func (m *Manager) WriteConfig(appName, content string) error {
	confPath := filepath.Join(m.SitesAvailable, appName+".conf")
	enabledPath := filepath.Join(m.SitesEnabled, appName+".conf")

	// Write the new config
	m.log.Debug("writing config", "app", appName, "path", confPath)
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Symlink to sites-enabled
	m.log.Debug("creating symlink", "src", confPath, "dst", enabledPath)
	os.Remove(enabledPath)
	if err := os.Symlink(confPath, enabledPath); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	// Test nginx config
	if err := run("nginx", "-t"); err != nil {
		m.log.Error("config test failed", "app", appName, "error", err.Error())
		// Revert on failure
		m.log.Warn("reverting config", "app", appName, "reason", "config test failed")
		m.revert(appName)
		return fmt.Errorf("nginx config test failed: %w", err)
	}
	m.log.Info("config test passed", "app", appName)

	// Commit to git
	gitRun(m.SitesAvailable, "add", appName+".conf")
	gitRun(m.SitesAvailable, "commit", "-m", fmt.Sprintf("deploy: %s config updated", appName))

	hash := m.currentHash()
	m.log.Info("config committed", "app", appName, "hash", hash)

	// Reload nginx
	return m.Reload()
}

// Reload reloads nginx gracefully.
func (m *Manager) Reload() error {
	if m.DevMode {
		m.log.Debug("[DEV MODE] skipping: nginx reload")
		return nil
	}

	if err := run("sudo", "systemctl", "reload", "nginx"); err != nil {
		m.log.Error("reload failed", "error", err.Error())
		return err
	}
	m.log.Info("reloaded successfully")
	return nil
}

// RemoveConfig removes an app's nginx config.
func (m *Manager) RemoveConfig(appName string) error {
	os.Remove(filepath.Join(m.SitesEnabled, appName+".conf"))
	confPath := filepath.Join(m.SitesAvailable, appName+".conf")
	os.Remove(confPath)
	gitRun(m.SitesAvailable, "add", "-A")
	gitRun(m.SitesAvailable, "commit", "-m", fmt.Sprintf("remove: %s config deleted", appName))
	return m.Reload()
}

func (m *Manager) currentHash() string {
	out, err := gitOutput(m.SitesAvailable, "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// ---- helpers ----

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

