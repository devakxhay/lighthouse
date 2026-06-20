package api

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/devakxhay/lighthouse/models"
)

// pullAndBuild pulls the latest git code and builds the app.
func (h *Handler) pullAndBuild(app *models.App) error {
	if app.AppDir == "" {
		return errors.New("app directory (app_dir) must be configured to pull and build")
	}

	// 1. Git pull
	if err := runCmd(app.AppDir, "git", "pull"); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}

	// 2. Build based on type
	switch app.Type {
	case models.AppTypeSpringBoot:
		if fileExists(filepath.Join(app.AppDir, "gradlew")) {
			return runCmd(app.AppDir, "./gradlew", "build", "-x", "test")
		} else if fileExists(filepath.Join(app.AppDir, "mvnw")) {
			return runCmd(app.AppDir, "./mvnw", "clean", "package", "-DskipTests")
		} else if fileExists(filepath.Join(app.AppDir, "pom.xml")) {
			return runCmd(app.AppDir, "mvn", "clean", "package", "-DskipTests")
		} else if fileExists(filepath.Join(app.AppDir, "build.gradle")) || fileExists(filepath.Join(app.AppDir, "build.gradle.kts")) {
			return runCmd(app.AppDir, "gradle", "build", "-x", "test")
		} else {
			return errors.New("spring-boot build tools not found (gradlew, mvnw, pom.xml, or build.gradle)")
		}

	case models.AppTypeNextJS:
		if err := runCmd(app.AppDir, "npm", "install"); err != nil {
			return fmt.Errorf("npm install: %w", err)
		}
		if err := runCmd(app.AppDir, "npm", "run", "build"); err != nil {
			return fmt.Errorf("npm run build: %w", err)
		}

	case models.AppTypeGo:
		if app.BinaryPath == "" {
			return errors.New("binary path (binary_path) must be configured to build a Go app")
		}
		_ = os.Remove(app.BinaryPath)
		if err := runCmd(app.AppDir, "go", "build", "-o", app.BinaryPath); err != nil {
			return fmt.Errorf("go build: %w", err)
		}

	default:
		return fmt.Errorf("unsupported app type: %s", app.Type)
	}

	return nil
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %s %v: %s: %w", name, args, string(out), err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
