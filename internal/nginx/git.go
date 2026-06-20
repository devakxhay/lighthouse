package nginx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CommitEntry struct {
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// Init sets up the sites-available dir as a git repo.
func (m *Manager) Init() error {
	if err := os.MkdirAll(m.SitesAvailable, 0755); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(m.SitesAvailable, ".git")); os.IsNotExist(err) {
		if err := gitRun(m.SitesAvailable, "init"); err != nil {
			return fmt.Errorf("git init: %w", err)
		}

		if err := gitRun(m.SitesAvailable, "config", "user.email", "lighthouse@local"); err != nil {
			return err
		}

		if err := gitRun(m.SitesAvailable, "config", "user.name", "Lighthouse"); err != nil {
			return err
		}
	}

	return nil
}

// History returns git log for an app's config.
func (m *Manager) History(appName string) ([]CommitEntry, error) {
	out, err := gitOutput(m.SitesAvailable, "log",
		"--format=%h|%ci|%s",
		"--", appName+".conf",
	)
	if err != nil {
		return nil, err
	}

	var entries []CommitEntry
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		t, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[1])
		entries = append(entries, CommitEntry{
			Hash:      parts[0],
			Timestamp: t,
			Message:   parts[2],
		})
	}
	return entries, nil
}

// Rollback restores a specific git commit for an app's config.
func (m *Manager) Rollback(appName, hash string) error {
	confFile := appName + ".conf"

	if err := gitRun(m.SitesAvailable, "checkout", hash, "--", confFile); err != nil {
		return fmt.Errorf("checkout failed: %w", err)
	}

	if err := run("nginx", "-t"); err != nil {
		gitRun(m.SitesAvailable, "checkout", "HEAD", "--", confFile)
		return fmt.Errorf("rollback config is invalid: %w", err)
	}

	gitRun(m.SitesAvailable, "add", confFile)
	gitRun(m.SitesAvailable, "commit", "-m",
		fmt.Sprintf("rollback: %s restored to %s", appName, hash))

	return m.Reload()
}

// revert restores the last committed version of the config.
func (m *Manager) revert(appName string) {
	gitRun(m.SitesAvailable, "checkout", "HEAD", "--", appName+".conf")
	run("nginx", "-t")
	run("systemctl", "reload", "nginx")
}

// ---- helpers ----

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
