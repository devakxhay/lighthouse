# Subprocess PATH Environment Inheriting and Binary Execution

**Date:** 2026-06-21

## Context
When running application builds dynamically through a background manager daemon like Lighthouse (which runs under systemd), commands like `npm install` are executed. However, `npm` is a wrapper script that runs with the shebang `#!/usr/bin/env node`.

## Problem
Even if we invoke `npm` via its absolute path `/usr/local/node/bin/npm`, it will try to find the `node` binary using `env node` which executes lookup on the process's standard environment `PATH`. Because `/usr/local/node/bin` was not in the systemd service unit's `PATH`, the shebang lookup failed with:
`npm install: command failed: ... /usr/bin/env: 'node': No such file or directory`

## Solution & Architectural Learning
To prevent this lookup failure:
1. **Dynamic PATH Injection**: On every build execution step, we retrieve all configured runtime binaries from the database, resolve their parent (bin) directories, and inject/prepend them into the `PATH` environment variable passed to `exec.Command`.
2. **System-wide Symlinking**: During the dependency installer execution (`lighthouse-install-bins`), we now also automatically create symlinks in `/usr/local/bin/` (e.g., `/usr/local/bin/node` pointing to `/usr/local/node/bin/node`) which is globally available to standard shells and services.
3. **Inheritance Base**: Ensure that when customizing environments for child command execution (like Go compiler configuration), we build upon our updated environment `env` rather than starting fresh from `os.Environ()`.
4. **Systemd Service PATH Environment**: The systemd service templates (`units/*.service`) for deployed applications must declare a dynamic `Environment=PATH={{.PathEnv}}` using paths of all configured runtimes. This ensures that when a service starts (e.g., executing `npm run start` which invokes node under the hood), the runner finds `node` successfully even without global symlinks.

This ensures any external shebangs or nested wrapper scripts (like npm packages calling `node`) always find their parent executors successfully.
