# Learning: Go Module Cache Fails for System Users with No Home Directory

**Date:** 2026-06-21  
**Error:** `go: could not create module cache: mkdir /home/lighthouse: permission denied`

---

## Root Cause

`useradd --system --no-create-home` without `--home-dir` records `/home/lighthouse`
in `/etc/passwd` as the HOME directory, but never creates the directory.

When the lighthouse daemon (running as `User=lighthouse`) calls `go build`, the Go
toolchain tries to create its module cache at `$HOME/.cache/go` → `mkdir /home/lighthouse`
→ **permission denied** (directory doesn't exist and lighthouse can't create it in /home).

## Fix: Three Layers

### 1. `install.sh` — declare a real, writable home

```bash
useradd --system \
    --gid lighthouse \
    --home-dir /var/lib/lighthouse \   # ← added
    --no-create-home \
    --shell /usr/sbin/nologin \
    lighthouse
# On re-installs / upgrades where user already exists with wrong home:
usermod --home /var/lib/lighthouse lighthouse 2>/dev/null || true
```

Also create Go-specific subdirs:
```bash
mkdir -p /var/lib/lighthouse/go        # GOPATH
mkdir -p /var/lib/lighthouse/go-cache  # GOCACHE
```
(ownership is set later by `chown -R lighthouse:lighthouse /var/lib/lighthouse`)

### 2. `install.sh` — systemd unit environment

```ini
Environment=HOME=/var/lib/lighthouse
Environment=GOPATH=/var/lib/lighthouse/go
Environment=GOCACHE=/var/lib/lighthouse/go-cache
```

This ensures every subprocess the daemon spawns (go, git, npm...) inherits correct,
writable paths.

### 3. `internal/api/build.go` — belt-and-suspenders in Go code

Go build explicitly sets the env on the subprocess using `Cfg.Dirs.Data` (config-driven,
works in dev mode too):

```go
goEnv := append(os.Environ(),
    "HOME="+h.Cfg.Dirs.Data,
    "GOPATH="+filepath.Join(h.Cfg.Dirs.Data, "go"),
    "GOCACHE="+filepath.Join(h.Cfg.Dirs.Data, "go-cache"),
)
runCmdWithEnv(app.AppDir, goEnv, goBin, "build", "-o", targetPath, path)
```

This prevents the bug even if the systemd unit is not updated (e.g. dev mode,
manual runs, future changes to the unit).

## General Rule

Any system service that runs build tools (go, cargo, pip, npm) as a no-home system
user MUST explicitly set `HOME`, `GOPATH`/`CARGO_HOME`/etc. in the unit file AND
in any `exec.Command` calls that invoke those tools.

Do NOT rely on the HOME from /etc/passwd for system users — it's often a phantom path.
