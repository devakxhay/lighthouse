# Binary Installer Design — Architectural Learnings

**Date:** 2026-06-21

## Context

Designed `lighthouse-install-bins`, a separate Go binary with Bubble Tea TUI for installing Java, Go, and Node runtimes needed by the Lighthouse deploy manager.

## Key Decisions & Rationale

### 1. Separate Binary, Not Subcommand
- `cmd/install-bins/main.go` compiles to `/usr/local/bin/lighthouse-install-bins`
- Keeps install code and daemon code completely separate
- Only the binary that needs root runs as root — daemon stays clean

### 2. Hybrid Install Strategy (not apt-only)
- apt-only is not viable: Go's apt version is outdated on Raspberry Pi / Debian
- Solution: **per-runtime `Installer` interface** with its own install strategy
  - Java → `apt-get install openjdk-21-jre-headless`
  - Go → official tarball from `go.dev/dl/?mode=json` API
  - Node → official tarball from `nodejs.org/dist/index.json` API
- This scales cleanly when new runtimes are added — just add a new file implementing the interface

### 3. `Installer` Interface Pattern
```go
type Installer interface {
    Name() string
    Label() string
    Detect() Status
    Install() (Status, error)
}
```
Per-runtime files: `java.go`, `go.go`, `node.go` — easy to add `python.go`, `dotnet.go` etc.

### 4. Version Resolution at Install Time
- Fetch latest stable from official JSON APIs — no hardcoded versions in code
- Go: `go.dev/dl/?mode=json`
- Node: `nodejs.org/dist/index.json` (filter for `lts != false`)
- Prevents stale version strings in the codebase

### 5. SHA256 Checksum Required
- Tarball downloads verified against official checksums before extraction
- Fail and clean up on mismatch — non-negotiable for system-level installs

### 6. Tarball Destinations
- Go → `/usr/local/go` (matches official go.dev install docs)
- Node → `/usr/local/node`
- npm is bundled with Node — write its bin_path to DB automatically when Node installs

### 7. DB Integration
- Uses existing `db.SetRuntimeOverride(name, path)` — no schema changes
- `overridden=true` prevents the daemon's startup `LookPath` from overwriting the saved path
- Daemon picks up new paths on restart — no live reload required

### 8. npm vs Node
- npm is bundled with Node — don't install separately
- In TUI: show as "Node.js LTS + npm" (single checkbox)
- After install: write both `node` and `npm` paths to DB

## Pitfalls Avoided
- **DO NOT** use apt-only for Go on Raspberry Pi — the package is too old
- **DO NOT** auto-run `install-bins` from `install.sh` — keep it user-initiated so initial install is fast and unattended
- **DO NOT** change the daemon's systemd unit PATH automatically — too risky, user re-runs `install.sh` if needed
