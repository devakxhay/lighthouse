package bins

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type NodeInstaller struct {
	DB DB
}

func (n *NodeInstaller) Name() string {
	return "node"
}

func (n *NodeInstaller) Label() string {
	return "Node.js LTS (includes npm)"
}

func (n *NodeInstaller) Detect() Status {
	// Check /usr/local/node/bin/node first
	path := "/usr/local/node/bin/node"
	if _, err := os.Stat(path); err == nil {
		return Status{Found: true, BinPath: path}
	}
	// Fallback to LookPath
	path, err := execLookPath("node")
	if err == nil {
		return Status{Found: true, BinPath: path}
	}
	return Status{Found: false}
}

func execLookPath(name string) (string, error) {
	// Simple lookup helper to avoid circular dependency
	importPath, ok := os.LookupEnv("PATH")
	if !ok {
		importPath = ""
	}
	if !strings.Contains(importPath, "/usr/local/node/bin") {
		// Temporarily append /usr/local/node/bin to PATH for detection in case it's not present yet
		os.Setenv("PATH", importPath+":/usr/local/node/bin")
		defer os.Setenv("PATH", importPath)
	}
	// Check if file exists in /usr/local/node/bin as well
	binPath := filepath.Join("/usr/local/node/bin", name)
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}
	// Fallback to standard LookPath
	return exec.LookPath(name)
}

type NodeRelease struct {
	Version string `json:"version"`
	LTS     any    `json:"lts"`
	Files   []string `json:"files"`
}

func (n *NodeInstaller) Install(ctx context.Context, logger LogFunc) (Status, error) {
	logger("Querying latest Node.js LTS release info...")
	req, err := http.NewRequestWithContext(ctx, "GET", "https://nodejs.org/dist/index.json", nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("fetch node release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Status{}, fmt.Errorf("nodejs.org returned status %d", resp.StatusCode)
	}

	var releases []NodeRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return Status{}, fmt.Errorf("decode node release info: %w", err)
	}

	// Find the latest LTS release
	var targetRelease *NodeRelease
	for i := range releases {
		r := &releases[i]
		if r.LTS != nil {
			isLTS := false
			if s, ok := r.LTS.(string); ok && s != "" {
				isLTS = true
			} else if b, ok := r.LTS.(bool); ok {
				isLTS = b
			}
			if isLTS {
				targetRelease = r
				break
			}
		}
	}

	if targetRelease == nil {
		return Status{}, fmt.Errorf("no LTS Node.js release found")
	}

	// Determine Node.js filename based on OS and architecture
	nodeArch := ""
	switch runtime.GOARCH {
	case "amd64":
		nodeArch = "linux-x64"
	case "arm64":
		nodeArch = "linux-arm64"
	case "arm":
		nodeArch = "linux-armv7l"
	case "386":
		nodeArch = "linux-x86"
	default:
		nodeArch = "linux-" + runtime.GOARCH
	}

	// Verify if the target file is in the list
	filenamePattern := fmt.Sprintf("node-%s-%s.tar.gz", targetRelease.Version, nodeArch)
	hasFile := false
	for _, f := range targetRelease.Files {
		if strings.Contains(f, nodeArch) {
			hasFile = true
			break
		}
	}
	if !hasFile {
		return Status{}, fmt.Errorf("target architecture %s is not supported by Node.js version %s", nodeArch, targetRelease.Version)
	}

	// We can construct the download URL and check SHASUMS256.txt
	downloadURL := fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-%s.tar.gz", targetRelease.Version, targetRelease.Version, nodeArch)
	sumsURL := fmt.Sprintf("https://nodejs.org/dist/%s/SHASUMS256.txt", targetRelease.Version)

	tempDir, err := os.MkdirTemp("", "lighthouse-node-*")
	if err != nil {
		return Status{}, err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, filenamePattern)

	logger(fmt.Sprintf("Downloading Node.js %s from %s...", targetRelease.Version, downloadURL))
	if err := downloadFile(ctx, logger, downloadURL, archivePath); err != nil {
		// Retry with tar.xz just in case tar.gz isn't preferred or available
		logger("Download failed. Attempting to download .tar.xz format...")
		filenamePattern = fmt.Sprintf("node-%s-%s.tar.xz", targetRelease.Version, nodeArch)
		downloadURL = fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-%s.tar.xz", targetRelease.Version, targetRelease.Version, nodeArch)
		archivePath = filepath.Join(tempDir, filenamePattern)
		if err := downloadFile(ctx, logger, downloadURL, archivePath); err != nil {
			return Status{}, fmt.Errorf("download Node.js archive failed: %w", err)
		}
	}

	logger("Fetching SHA256 checksums file...")
	sumsReq, err := http.NewRequestWithContext(ctx, "GET", sumsURL, nil)
	if err != nil {
		return Status{}, err
	}
	sumsResp, err := http.DefaultClient.Do(sumsReq)
	if err != nil {
		return Status{}, fmt.Errorf("fetch node checksums: %w", err)
	}
	defer sumsResp.Body.Close()

	if sumsResp.StatusCode != http.StatusOK {
		return Status{}, fmt.Errorf("fetch checksums returned status %d", sumsResp.StatusCode)
	}

	expectedSHA := ""
	scanner := bufio.NewScanner(sumsResp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == filenamePattern {
			expectedSHA = parts[0]
			break
		}
	}

	if expectedSHA == "" {
		return Status{}, fmt.Errorf("checksum not found for %s in SHASUMS256.txt", filenamePattern)
	}

	logger("Verifying SHA256 checksum...")
	if err := verifySHA(archivePath, expectedSHA); err != nil {
		return Status{}, fmt.Errorf("checksum validation failed: %w", err)
	}
	logger("Checksum verified successfully.")

	logger("Extracting archive...")
	extractDir := filepath.Join(tempDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return Status{}, err
	}

	if err := runCmd(ctx, logger, "tar", "-C", extractDir, "-xf", archivePath); err != nil {
		return Status{}, fmt.Errorf("extraction failed: %w", err)
	}

	// Find the extracted subdirectory
	files, err := os.ReadDir(extractDir)
	if err != nil {
		return Status{}, err
	}
	if len(files) == 0 {
		return Status{}, fmt.Errorf("no files extracted")
	}
	subName := files[0].Name()

	logger("Cleaning up existing /usr/local/node if any...")
	if err := os.RemoveAll("/usr/local/node"); err != nil {
		logger(fmt.Sprintf("warning: failed to remove /usr/local/node: %v", err))
	}

	logger("Moving Node.js to /usr/local/node...")
	if err := os.MkdirAll("/usr/local", 0755); err != nil {
		return Status{}, err
	}

	srcPath := filepath.Join(extractDir, subName)
	if err := runCmd(ctx, logger, "mv", srcPath, "/usr/local/node"); err != nil {
		// Fallback copy/move if mv command fails across filesystems
		return Status{}, fmt.Errorf("failed to move node to /usr/local/node: %w", err)
	}

	status := n.Detect()
	if !status.Found {
		return Status{}, fmt.Errorf("node binary not found after successful installation")
	}
	status.Version = targetRelease.Version

	logger("Creating symlinks for node and npm in /usr/local/bin...")
	_ = os.Remove("/usr/local/bin/node")
	if err := os.Symlink(status.BinPath, "/usr/local/bin/node"); err != nil {
		logger(fmt.Sprintf("warning: failed to create symlink for node: %v", err))
	}

	npmPath := "/usr/local/node/bin/npm"
	if _, err := os.Stat(npmPath); err == nil {
		logger(fmt.Sprintf("Bundled npm detected at: %s", npmPath))

		_ = os.Remove("/usr/local/bin/npm")
		if err := os.Symlink(npmPath, "/usr/local/bin/npm"); err != nil {
			logger(fmt.Sprintf("warning: failed to create symlink for npm: %v", err))
		}

		if n.DB != nil {
			logger("Saving npm path to database...")
			if err := n.DB.SetRuntimeOverride("npm", npmPath); err != nil {
				return Status{}, fmt.Errorf("failed to save npm path to DB: %w", err)
			}
		}
	} else {
		logger("warning: npm not found in Node.js bundle")
	}

	npxPath := "/usr/local/node/bin/npx"
	if _, err := os.Stat(npxPath); err == nil {
		logger(fmt.Sprintf("Bundled npx detected at: %s", npxPath))

		_ = os.Remove("/usr/local/bin/npx")
		if err := os.Symlink(npxPath, "/usr/local/bin/npx"); err != nil {
			logger(fmt.Sprintf("warning: failed to create symlink for npx: %v", err))
		}

		if n.DB != nil {
			logger("Saving npx path to database...")
			if err := n.DB.SetRuntimeOverride("npx", npxPath); err != nil {
				return Status{}, fmt.Errorf("failed to save npx path to DB: %w", err)
			}
		}
	} else {
		logger("warning: npx not found in Node.js bundle")
	}

	logger(fmt.Sprintf("Node.js installed successfully at: %s (%s)", status.BinPath, status.Version))
	return status, nil
}
