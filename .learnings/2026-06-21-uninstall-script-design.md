# System Uninstall Script Design & Robustness

When writing teardown/uninstall scripts (`uninstall.sh`) that require `sudo` privileges to clean up system-level services, configuration, and user permissions, several critical bash and system-level issues must be addressed.

## Key Learnings

### 1. Robust Bash Globbing
* **Issue:** Wildcard matches (globbing) like `/etc/systemd/system/lighthouse-*.service` or `/etc/nginx/sites-enabled/lighthouse-*` do not expand if no matching files exist. Instead, the loop executes once with the literal pattern string, causing errors.
* **Fix:** Add a protective existence check `[ -e "$file" ] || continue` inside loops, or configure `shopt -s nullglob` to prevent execution when no files match.

### 2. Correct Home Directory Resolution Under Sudo
* **Issue:** When running a script under `sudo`, the `~` symbol or `$HOME` variable resolves to `/root/`. If backing up critical user assets (like SSL CA certs), they end up in the root home directory, which is inaccessible and hidden from the regular user.
* **Fix:** Safely inspect the `$SUDO_USER` environment variable. If set, query `getent passwd "$SUDO_USER"` to locate the actual user's home directory and target that directory for backup. Ensure permissions are updated (`chown`) so the backup is owned by the calling user:
  ```bash
  if [ -n "$SUDO_USER" ]; then
      USER_HOME=$(getent passwd "$SUDO_USER" | cut -d: -f6)
  else
      USER_HOME=$HOME
  fi
  ```

### 3. Restoring Changed System Permissions
* **Issue:** Installer scripts sometimes modify standard directory ownership/permissions (e.g. `chown root:lighthouse /etc/systemd/system` and `chmod g+wx`). Uninstall scripts must restore these directory permissions to defaults to prevent leaving behind security holes (e.g., privilege escalation pathways via group write access).
