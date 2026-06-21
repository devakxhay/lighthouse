package nginx

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type Manager struct {
	SitesAvailable string
	SitesEnabled   string
	DevMode        bool
}

// WriteConfig writes a new nginx config, tests it, and commits on success.
// On failure, it reverts and returns an error.
func (m *Manager) WriteConfig(appName, content string) error {
	confPath := filepath.Join(m.SitesAvailable, appName+".conf")
	enabledPath := filepath.Join(m.SitesEnabled, appName+".conf")

	// Write the new config
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Symlink to sites-enabled
	os.Remove(enabledPath)
	if err := os.Symlink(confPath, enabledPath); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	// Test nginx config
	if err := run("nginx", "-t"); err != nil {
		// Revert on failure
		m.revert(appName)
		return fmt.Errorf("nginx config test failed: %w", err)
	}

	// Commit to git
	gitRun(m.SitesAvailable, "add", appName+".conf")
	gitRun(m.SitesAvailable, "commit", "-m", fmt.Sprintf("deploy: %s config updated", appName))

	// Reload nginx
	return m.Reload()
}

// Reload reloads nginx gracefully.
func (m *Manager) Reload() error {
	if m.DevMode {
		log.Printf("[DEV] nginx reload skipped")
		return nil
	}

	return run("systemctl", "reload", "nginx")
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

// ---- helpers ----

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}
