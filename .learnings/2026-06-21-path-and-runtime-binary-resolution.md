# Bug Fix: Path Resolution for Deploy Build Step

## Context
When running Lighthouse under a dedicated systemd service, standard build tool executables like `go` and `npm` could not be located during deployment, resulting in `exec: "go": executable file not found in $PATH` errors. This happened even when the user manually overrode the path in the Settings UI or when the binary existed in `/usr/local/go/bin/go`.

## Cause
1. Systemd runs services with a restricted `PATH` environment variable by default, which excludes typical custom installation directories (e.g., `/usr/local/go/bin`).
2. The Go-side build logic (`internal/api/build.go`) previously used hardcoded strings (e.g., `"go"`, `"npm"`) instead of retrieving the paths from the runtimes database.

## Solution

### 1. Database-Configured Paths for Builds
Updated the build handler in [internal/api/build.go](file:///home/akxhd/lighthouse/internal/api/build.go) to fetch user-configured overrides from the database before performing builds:
```go
goBin := "go"
if rt, _ := h.DB.GetRuntime("go"); rt != nil && rt.BinPath != "" {
    goBin = rt.BinPath
}
```

### 2. Systemd PATH Extension
Updated the systemd service template in [install.sh](file:///home/akxhd/lighthouse/install.sh) to include standard execution folders in its environment settings:
```ini
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin
```
This guarantees that build/runtime binaries are in scope even if the database has not been overridden by the user yet.
