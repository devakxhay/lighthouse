package bins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type GoInstaller struct{}

func (g *GoInstaller) Name() string {
	return "go"
}

func (g *GoInstaller) Label() string {
	return "Go (latest stable via go.dev)"
}

func (g *GoInstaller) Detect() Status {
	// Check /usr/local/go/bin/go first
	path := "/usr/local/go/bin/go"
	if _, err := os.Stat(path); err == nil {
		return Status{Found: true, BinPath: path}
	}
	// Fallback to LookPath
	path, err := exec.LookPath("go")
	if err == nil {
		return Status{Found: true, BinPath: path}
	}
	return Status{Found: false}
}

type GoRelease struct {
	Version string   `json:"version"`
	Stable  bool     `json:"stable"`
	Files   []GoFile `json:"files"`
}

type GoFile struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Kind     string `json:"kind"`
}

func (g *GoInstaller) Install(ctx context.Context, logger LogFunc) (Status, error) {
	logger("Querying latest Go release info from go.dev...")
	req, err := http.NewRequestWithContext(ctx, "GET", "https://go.dev/dl/?mode=json", nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Status{}, fmt.Errorf("go.dev returned status %d", resp.StatusCode)
	}

	var releases []GoRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return Status{}, fmt.Errorf("decode release info: %w", err)
	}

	if len(releases) == 0 {
		return Status{}, fmt.Errorf("no Go releases returned")
	}

	// Find the latest stable release
	var targetRelease *GoRelease
	for i := range releases {
		if releases[i].Stable {
			targetRelease = &releases[i]
			break
		}
	}
	if targetRelease == nil {
		return Status{}, fmt.Errorf("no stable Go release found")
	}

	targetOS := "linux"
	targetArch := runtime.GOARCH
	if targetArch == "arm" {
		targetArch = "armv6l" // Go's download suffix for 32-bit ARM Linux
	}

	var targetFile *GoFile
	for i := range targetRelease.Files {
		f := &targetRelease.Files[i]
		if f.OS == targetOS && f.Arch == targetArch && f.Kind == "archive" {
			targetFile = f
			break
		}
	}

	if targetFile == nil {
		return Status{}, fmt.Errorf("no archive file found for OS=%s, Arch=%s in Go %s", targetOS, targetArch, targetRelease.Version)
	}

	downloadURL := fmt.Sprintf("https://go.dev/dl/%s", targetFile.Filename)
	tempDir, err := os.MkdirTemp("", "lighthouse-go-*")
	if err != nil {
		return Status{}, err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, targetFile.Filename)
	logger(fmt.Sprintf("Downloading Go %s from %s...", targetRelease.Version, downloadURL))

	if err := downloadFile(ctx, logger, downloadURL, archivePath); err != nil {
		return Status{}, fmt.Errorf("download failed: %w", err)
	}

	logger("Verifying SHA256 checksum...")
	if err := verifySHA(archivePath, targetFile.SHA256); err != nil {
		return Status{}, fmt.Errorf("checksum validation failed: %w", err)
	}
	logger("Checksum verified successfully.")

	logger("Cleaning up existing /usr/local/go if any...")
	if err := os.RemoveAll("/usr/local/go"); err != nil {
		logger(fmt.Sprintf("warning: failed to remove /usr/local/go: %v", err))
	}

	logger("Extracting archive to /usr/local...")
	// Run tar -C /usr/local -xzf archivePath
	if err := runCmd(ctx, logger, "tar", "-C", "/usr/local", "-xzf", archivePath); err != nil {
		return Status{}, fmt.Errorf("extraction failed: %w", err)
	}

	status := g.Detect()
	if !status.Found {
		return Status{}, fmt.Errorf("go binary not found after successful extraction")
	}
	status.Version = targetRelease.Version

	logger("Creating symlink for go in /usr/local/bin...")
	_ = os.Remove("/usr/local/bin/go")
	if err := os.Symlink(status.BinPath, "/usr/local/bin/go"); err != nil {
		logger(fmt.Sprintf("warning: failed to create symlink for go: %v", err))
	}

	logger(fmt.Sprintf("Go installed successfully at: %s (%s)", status.BinPath, status.Version))
	return status, nil
}

func downloadFile(ctx context.Context, logger LogFunc, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	// Simple copy with progress logging every 10MB
	buf := make([]byte, 32*1024)
	var written int64
	var lastLogged int64
	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := out.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
		if written-lastLogged >= 10*1024*1024 {
			logger(fmt.Sprintf("Downloaded %d MB...", written/(1024*1024)))
			lastLogged = written
		}
	}

	return err
}

func verifySHA(filePath, expectedHex string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualHex := hex.EncodeToString(h.Sum(nil))
	if actualHex != expectedHex {
		return fmt.Errorf("expected %s, got %s", expectedHex, actualHex)
	}
	return nil
}
