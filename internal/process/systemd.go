package process

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Manager struct {
	UnitsDir string
	DevMode  bool
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
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	return run("systemctl", "daemon-reload")
}

// RemoveUnit stops and removes the systemd unit for an app.
func (m *Manager) RemoveUnit(appName string) error {
	svc := serviceName(appName)
	run("systemctl", "stop", svc)
	run("systemctl", "disable", svc)
	os.Remove(filepath.Join(m.UnitsDir, "lighthouse-"+appName+".service"))
	return run("systemctl", "daemon-reload")
}

// Enable enables the service to start on boot.
func (m *Manager) Enable(appName string) error {
	return run("systemctl", "enable", serviceName(appName))
}

// Start starts the service.
func (m *Manager) Start(appName string) error {
	if m.DevMode {
		log.Printf("[DEV] systemctl start lighthouse-%s.service", appName)
		return nil
	}

	return run("systemctl", "start", serviceName(appName))
}

// Stop stops the service.
func (m *Manager) Stop(appName string) error {
	return run("systemctl", "stop", serviceName(appName))
}

// Restart restarts the service.
func (m *Manager) Restart(appName string) error {
	return run("systemctl", "restart", serviceName(appName))
}

// Status returns the current status of the service.
func (m *Manager) Status(appName string) (*ServiceStatus, error) {
	out, err := exec.Command("systemctl", "show", serviceName(appName),
		"--property=ActiveState,SubState,LoadState",
		"--no-pager",
	).Output()
	if err != nil {
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
