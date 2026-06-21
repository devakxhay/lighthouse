package process

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Manager struct {
	UnitsDir string
	DevMode  bool
	log      *slog.Logger
}

func NewManager(unitsDir string, devMode bool, logger *slog.Logger) *Manager {
	return &Manager{
		UnitsDir: unitsDir,
		DevMode:  devMode,
		log:      logger.With(slog.String("component", "systemd")),
	}
}

type ServiceStatus struct {
	Active string `json:"active"` // active, inactive, failed
	Sub    string `json:"sub"`    // running, dead, exited
	Load   string `json:"load"`
}

// WriteUnit writes a systemd unit file for the app.
func (m *Manager) WriteUnit(appName, content string) error {
	if err := os.MkdirAll(m.UnitsDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(m.UnitsDir, "lighthouse-"+appName+".service")
	m.log.Debug("writing unit file", "app", appName, "path", path)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	if err := run("sudo", "systemctl", "daemon-reload"); err != nil {
		return err
	}
	m.log.Info("daemon-reload complete")
	return nil
}

// RemoveUnit stops and removes the systemd unit for an app.
func (m *Manager) RemoveUnit(appName string) error {
	svc := serviceName(appName)
	run("sudo", "systemctl", "stop", svc)
	run("sudo", "systemctl", "disable", svc)
	os.Remove(filepath.Join(m.UnitsDir, "lighthouse-"+appName+".service"))
	m.log.Warn("unit removed", "app", appName)
	if err := run("sudo", "systemctl", "daemon-reload"); err != nil {
		return err
	}
	m.log.Info("daemon-reload complete")
	return nil
}

// Enable enables the service to start on boot.
func (m *Manager) Enable(appName string) error {
	if err := run("sudo", "systemctl", "enable", serviceName(appName)); err != nil {
		return err
	}
	m.log.Info("service enabled", "app", appName)
	return nil
}

// Start starts the service.
func (m *Manager) Start(appName string) error {
	if m.DevMode {
		m.log.Debug("[DEV MODE] skipping: systemctl start " + serviceName(appName))
		return nil
	}

	if err := run("sudo", "systemctl", "start", serviceName(appName)); err != nil {
		m.log.Error("start failed", "app", appName, "error", err.Error())
		return err
	}
	m.log.Info("service started", "app", appName)
	return nil
}

// Stop stops the service.
func (m *Manager) Stop(appName string) error {
	if err := run("sudo", "systemctl", "stop", serviceName(appName)); err != nil {
		return err
	}
	m.log.Info("service stopped", "app", appName)
	return nil
}

// Restart restarts the service.
func (m *Manager) Restart(appName string) error {
	if err := run("sudo", "systemctl", "restart", serviceName(appName)); err != nil {
		return err
	}
	m.log.Info("service restarted", "app", appName)
	return nil
}

// Status returns the current status of the service.
func (m *Manager) Status(appName string) (*ServiceStatus, error) {
	out, err := exec.Command("systemctl", "show", serviceName(appName),
		"--property=ActiveState,SubState,LoadState",
		"--no-pager",
	).Output()
	if err != nil {
		m.log.Debug("status check failed", "app", appName, "error", err.Error())
		return &ServiceStatus{Active: "unknown", Sub: "unknown"}, nil
	}

	status := &ServiceStatus{}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "ActiveState":
			status.Active = parts[1]
		case "SubState":
			status.Sub = parts[1]
		case "LoadState":
			status.Load = parts[1]
		}
	}
	m.log.Debug("status check", "app", appName, "active", status.Active, "sub", status.Sub)
	return status, nil
}

// Logs returns the last N lines of journal logs for the app.
func (m *Manager) Logs(appName string, lines int) (string, error) {
	out, err := exec.Command("journalctl",
		"-u", serviceName(appName),
		"-n", fmt.Sprintf("%d", lines),
		"--no-pager",
		"--output=short-iso",
	).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func serviceName(appName string) string {
	return "lighthouse-" + appName + ".service"
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

