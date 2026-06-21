package api

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devakxhay/lighthouse/models"
)

// pullAndBuild pulls the latest git code and builds the app.
func (h *Handler) pullAndBuild(app *models.App) error {
	if app.GitURL == "" {
		return errors.New("git URL (git_url) is required to deploy")
	}

	// If no app directory provided, create a default directory under lighthouse data dir
	if app.AppDir == "" {
		app.AppDir = filepath.Join(h.Cfg.Dirs.Data, "apps", app.Name)
	}

	// Build custom environment appending all configured runtime bin paths to PATH
	var customPaths []string
	runtimes, err := h.DB.ListRuntimes()
	if err == nil {
		for _, rt := range runtimes {
			if rt.BinPath != "" {
				customPaths = append(customPaths, filepath.Dir(rt.BinPath))
			}
		}
	}

	env := os.Environ()
	pathVar := ""
	pathIndex := -1
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			pathVar = e[5:]
			pathIndex = i
			break
		}
	}
	for _, cp := range customPaths {
		if pathVar == "" {
			pathVar = cp
		} else if !strings.Contains(pathVar, cp) {
			pathVar = cp + ":" + pathVar
		}
	}
	if pathIndex != -1 {
		env[pathIndex] = "PATH=" + pathVar
	} else {
		env = append(env, "PATH="+pathVar)
	}

	// 1. Clone or Pull
	gitDir := filepath.Join(app.AppDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(app.AppDir), 0755); err != nil {
			return fmt.Errorf("create parent dir: %w", err)
		}
		// Clone repository
		h.log.Info("cloning git repository", "app", app.Name, "url", app.GitURL)
		if err := runCmdWithEnv(filepath.Dir(app.AppDir), env, "git", "clone", app.GitURL, filepath.Base(app.AppDir)); err != nil {
			return fmt.Errorf("git clone: %w", err)
		}
	} else {
		// Pull latest
		h.log.Info("pulling git repository", "app", app.Name)
		if err := runCmdWithEnv(app.AppDir, env, "git", "pull"); err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
	}

	// 2. Build based on type
	h.log.Info("building application", "app", app.Name, "type", string(app.Type))
	switch app.Type {
	case models.AppTypeSpringBoot:
		var buildErr error
		if fileExists(filepath.Join(app.AppDir, "gradlew")) {
			buildErr = runCmdWithEnv(app.AppDir, env, "./gradlew", "build", "-x", "test")
		} else if fileExists(filepath.Join(app.AppDir, "mvnw")) {
			buildErr = runCmdWithEnv(app.AppDir, env, "./mvnw", "clean", "package", "-DskipTests")
		} else if fileExists(filepath.Join(app.AppDir, "pom.xml")) {
			buildErr = runCmdWithEnv(app.AppDir, env, "mvn", "clean", "package", "-DskipTests")
		} else if fileExists(filepath.Join(app.AppDir, "build.gradle")) || fileExists(filepath.Join(app.AppDir, "build.gradle.kts")) {
			buildErr = runCmdWithEnv(app.AppDir, env, "gradle", "build", "-x", "test")
		} else {
			return errors.New("spring-boot build tools not found (gradlew, mvnw, pom.xml, or build.gradle)")
		}

		if buildErr != nil {
			return fmt.Errorf("spring-boot build: %w", buildErr)
		}

		// Find the built jar file
		jarPath, err := findSpringBootJar(app.AppDir)
		if err != nil {
			return err
		}
		app.BinaryPath = jarPath

	case models.AppTypeNextJS:
		npmBin := "npm"
		if rt, _ := h.DB.GetRuntime("npm"); rt != nil && rt.BinPath != "" {
			npmBin = rt.BinPath
		}
		if err := runCmdWithEnv(app.AppDir, env, npmBin, "install"); err != nil {
			return fmt.Errorf("npm install: %w", err)
		}
		if err := runCmdWithEnv(app.AppDir, env, npmBin, "run", "build"); err != nil {
			return fmt.Errorf("npm run build: %w", err)
		}

	case models.AppTypeGo:
		app.BinaryPath = filepath.Join(app.AppDir, app.Name)
		_ = os.Remove(app.BinaryPath)

		entryPoint := app.EntryPoint
		if entryPoint == "" {
			entryPoint = "main.go"
		}

		path, _ := filepath.Abs(filepath.Join(app.AppDir, entryPoint))
		targetPath, _ := filepath.Abs(app.BinaryPath)

		goBin := "go"
		if rt, _ := h.DB.GetRuntime("go"); rt != nil && rt.BinPath != "" {
			goBin = rt.BinPath
		}

		// Explicitly set Go env to writable dirs under the data directory.
		// The lighthouse system user has no real HOME (/home/lighthouse doesn't exist),
		// so without this go build fails: "could not create module cache".
		goEnv := append(env,
			"HOME="+h.Cfg.Dirs.Data,
			"GOPATH="+filepath.Join(h.Cfg.Dirs.Data, "go"),
			"GOCACHE="+filepath.Join(h.Cfg.Dirs.Data, "go-cache"),
		)
		if err := runCmdWithEnv(app.AppDir, goEnv, goBin, "build", "-o", targetPath, path); err != nil {
			return fmt.Errorf("go build: %w", err)
		}

	default:
		return fmt.Errorf("unsupported app type: %s", app.Type)
	}

	return nil
}

func findSpringBootJar(appDir string) (string, error) {
	patterns := []string{
		filepath.Join(appDir, "target", "*.jar"),
		filepath.Join(appDir, "build", "libs", "*.jar"),
	}

	var candidate string
	var maxSize int64

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			base := filepath.Base(match)
			if containsAny(base, "-sources", "-javadoc", "-plain", ".original") {
				continue
			}
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			if info.Size() > maxSize {
				maxSize = info.Size()
				candidate = match
			}
		}
	}

	if candidate == "" {
		return "", errors.New("could not find any built Spring Boot jar file in target/ or build/libs/")
	}
	return candidate, nil
}

func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
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

func runCmdWithEnv(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
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
