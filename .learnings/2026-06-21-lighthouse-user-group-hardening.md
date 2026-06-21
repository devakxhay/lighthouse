# Security Hardening: Running Lighthouse under Dedicated User and Group

## Context
Originally, Lighthouse ran with full root privileges. To harden the security posture, the architecture was redesigned to run the service under a dedicated non-privileged `lighthouse` user and group.

## Architectural Changes & Learnings

### 1. File and Directory Permissions
To operate without full root privileges, the `lighthouse` user requires specific access:
- **Full Ownership (`lighthouse:lighthouse`):** `/etc/lighthouse/` (configuration and CA credentials) and `/var/lib/lighthouse/` (SQLite database).
- **Group Ownership/Permissions (`root:lighthouse`):**
  - `/etc/nginx/sites-available/` and `/etc/nginx/sites-enabled/` are set to `g+rwx`.
  - `/etc/systemd/system/` is set to `g+wx`. Keeping the read bit `r` unset for the group prevents non-root users from listing the contents of the directory (security hardening), while allowing them to write and update systemd unit files.

### 2. Privilege Escalation via Sudoers Allowlist
Systemd daemon reloads, starts, stops, and reloads of Nginx require privileges. We configure a targeted sudoers ruleset allowing the `lighthouse` user to run `systemctl` commands prefixing only `lighthouse-*` services and Nginx reloads:
```sudoers
lighthouse ALL=(ALL) NOPASSWD: \
    /bin/systemctl daemon-reload, \
    /bin/systemctl start lighthouse-*, \
    /bin/systemctl stop lighthouse-*, \
    /bin/systemctl restart lighthouse-*, \
    /bin/systemctl enable lighthouse-*, \
    /bin/systemctl disable lighthouse-*, \
    /bin/systemctl reload nginx
```
As a consequence, the application codebase (specifically in `internal/process/systemd.go` and `internal/nginx/manager.go`) was modified to prefix the administrative commands with `sudo`.

### 3. Log Retrieval Permissions
Fetching service logs via `journalctl -u lighthouse-*` fails for standard non-root users. To address this, the `lighthouse` user is added to the `systemd-journal` group, granting it log access without requiring `sudo`.
