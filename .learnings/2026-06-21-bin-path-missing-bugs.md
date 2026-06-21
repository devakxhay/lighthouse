# Bug Analysis: Bin Path Missing in $PATH on Deploy

**Date:** 2026-06-21  
**Files Affected:** `install.sh`, `internal/api/runtimes.go`, `internal/process/systemd.go`, `internal/templates/units/`

---

## Root Causes Found

### 1. `sudoers` hardcodes `/bin/systemctl` — mismatch on modern distros

`install.sh` writes:
```
/bin/systemctl daemon-reload, ...
```
But on modern Raspberry Pi OS / Debian / Ubuntu, `systemctl` lives at `/usr/bin/systemctl`
(even if `/bin` → `/usr/bin` is a symlink, the absolute path in sudoers must match what `sudo` resolves).

`process/systemd.go` calls `sudo systemctl ...` (no absolute path), so sudo looks it up via PATH — but the sudoers rule only allows `/bin/systemctl`. On systems where the symlink resolution differs, sudo denies the call silently.

**Fix:** Write both paths in sudoers OR use `/usr/bin/systemctl` (canonical on systemd distros).

### 2. `runtime.Detector` created in `runtimes.go` without `cfg` — PATH never augmented

In `internal/api/runtimes.go`, `DetectRuntimes` creates a bare Detector:
```go
detector := &runtime.Detector{DB: h.DB}
```
This omits `cfg` (and `log`). The `detect.go` code checks `d.cfg.RuntimePaths` first — with `cfg == nil` this will **panic** (nil pointer dereference) on any config-defined runtime path. Even without a panic, the config-overridden paths are silently skipped.

Startup uses `runtime.NewDetector(database, cfg, log)` correctly, but the HTTP-triggered re-detect does not.

**Fix:** Use `runtime.NewDetector(h.DB, h.Cfg, h.log)` in `DetectRuntimes`.

### 3. Systemd unit templates have no `Environment=PATH=...` — deployed apps inherit a bare systemd PATH

All three unit templates (`0001_spring_boot.service`, `0002_next_js.service`, `0003_go.service`) use absolute bin paths from the DB (`{{.JavaBin}}`, `{{.NpmBin}}`, `{{.GoBin}}`), but the spawned process environment has no `PATH` override. Any app that itself shells out (e.g. a Go app calling `exec.Command("git", ...)`) will fail because systemd default PATH is `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`.

Lighthouse's own service unit in `install.sh` correctly sets:
```
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin
```
But the app unit templates omit this entirely.

**Fix:** Add `Environment=PATH=...` (including the runtime's parent dir) to each `.service` template, or pass `{{.GoBin | dir}}` etc. and prepend it.

### 4. Go app `EntryPoint` variable shadowed — wrong path passed to `go build`

In `internal/api/build.go` (AppTypeGo branch):
```go
entryPoint := app.EntryPoint
if entryPoint == "" {
    entryPoint = "main.go"
}

path, _ := filepath.Abs(filepath.Join(app.AppDir, app.EntryPoint)) // ← uses app.EntryPoint, NOT local entryPoint
```
The local `entryPoint` variable (with the `"main.go"` fallback) is assigned but never used. The `filepath.Abs` call reads `app.EntryPoint` again — which is still `""`. This passes an empty-string path to `go build -o <target> ""`, which silently builds nothing or errors. The binary is never created, so `{{.BinaryPath}}` in the systemd unit points at a non-existent file → service fails to start.

**Fix:**
```go
path, _ := filepath.Abs(filepath.Join(app.AppDir, entryPoint))
```

### 5. `chown lighthouse:lighthouse /usr/local/bin/lighthouse` — wrong owner prevents binary execution

`install.sh` line 166 changes the binary owner to `lighthouse:lighthouse`. When systemd starts the service as `User=lighthouse`, the binary is owned by `lighthouse` but the *execute* bit is fine. However if the binary is later updated (re-run `install.sh`), `go build` runs as root and re-writes the file — but `chown` is only called after build. Between builds, if somehow the permissions end up 700, the system user can't exec it. Not a hard bug but a hardening gap; `chmod 755` should follow `chown`.

---

## Summary Table

| # | File | Bug | Severity |
|---|------|-----|----------|
| 1 | `install.sh` | sudoers uses `/bin/systemctl` not `/usr/bin/systemctl` | High |
| 2 | `internal/api/runtimes.go` | `Detector` created without `cfg` → nil-ptr panic + PATH overrides ignored | High |
| 3 | `internal/templates/units/*.service` | No `Environment=PATH=` in app unit files | Medium |
| 4 | `internal/api/build.go` | `entryPoint` local var shadowed; `app.EntryPoint` (empty) used for `go build` | High |
| 5 | `install.sh` | Missing `chmod 755` after `chown` on binary | Low |
| **6** | **`internal/templates/units/*.service`** | **No `User=`/`Group=` — deployed apps run as root** | **Critical** |
| **7** | **`install.sh`** | **Binary `chown lighthouse:lighthouse` — user can overwrite its own binary** | **High** |

---

## Least-Privilege Bugs (Addendum 2026-06-21)

The `lighthouse` system user/group exists specifically to sandbox the deploy manager.
Two violations were found where the privilege boundary was broken.

### 6. App unit templates missing `User=`/`Group=` — deployed apps run as **root**

**Files:** `0001_spring_boot.service`, `0002_next_js.service`, `0003_go.service`

Systemd's default when `User=` is omitted is `root`. Every deployed application
launched by Lighthouse ran as root despite the lighthouse user/group being set up
specifically to prevent this. A compromised deployed app would have full root access.

**Fix:** Added `User=lighthouse` and `Group=lighthouse` to all three templates,
matching the `lighthouse.service` itself.

### 7. Binary owned by `lighthouse:lighthouse` — privilege self-escalation path

**File:** `install.sh:166`

`chown lighthouse:lighthouse /usr/local/bin/lighthouse` means the lighthouse user
(which runs the live daemon) owns its own executable. Since the process runs as
that user, a code-execution bug in the daemon, or a path-traversal in the API,
could overwrite the binary with arbitrary code that then re-executes as the same
user on next `systemctl restart`. This is a privilege persistence / backdoor vector.

**Fix:** `chown root:lighthouse /usr/local/bin/lighthouse && chmod 750` — root
owns and can only be replaced by root (i.e., a re-run of `install.sh`), but
the lighthouse group retains read+execute.

