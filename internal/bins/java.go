package bins

import (
	"context"
	"fmt"
	"os/exec"
)

type JavaInstaller struct{}

func (j *JavaInstaller) Name() string {
	return "java"
}

func (j *JavaInstaller) Label() string {
	return "Java 21 (OpenJDK via apt)"
}

func (j *JavaInstaller) Detect() Status {
	path, err := exec.LookPath("java")
	if err != nil {
		return Status{Found: false}
	}
	return Status{Found: true, BinPath: path}
}

func (j *JavaInstaller) Install(ctx context.Context, logger LogFunc) (Status, error) {
	logger("Updating package lists (apt-get update)...")
	if err := runCmd(ctx, logger, "apt-get", "update"); err != nil {
		return Status{}, fmt.Errorf("apt-get update failed: %w", err)
	}

	logger("Installing openjdk-21-jre-headless...")
	if err := runCmd(ctx, logger, "apt-get", "install", "-y", "openjdk-21-jre-headless"); err != nil {
		return Status{}, fmt.Errorf("apt-get install openjdk-21-jre-headless failed: %w", err)
	}

	status := j.Detect()
	if !status.Found {
		return Status{}, fmt.Errorf("java not found in PATH after install")
	}

	logger(fmt.Sprintf("Java installed at: %s", status.BinPath))
	return status, nil
}
