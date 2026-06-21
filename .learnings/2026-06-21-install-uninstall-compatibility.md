# Install / Uninstall Compatibility Analysis

**Date:** 2026-06-21

---

## Mismatches Found

### 1. nginx reload uses `sudo systemctl reload nginx` — but uninstall uses plain `nginx -t && systemctl reload nginx`

**Install:** `nginx/manager.go` calls `sudo systemctl reload nginx`  
**Uninstall (line 67):** `nginx -t && systemctl reload nginx || true` (no `sudo`)

Since uninstall.sh runs as root this is benign at uninstall time. But it is inconsistent.
Not a functional bug in uninstall.

### 2. `/etc/systemd/system` permission mismatch — install sets `g+wx`, uninstall restores `755`

**Install (line 138-139):**
```
chown root:lighthouse /etc/systemd/system
chmod g+wx /etc/systemd/system   # i.e. 775 (drwxrwxr-x)
```

**Uninstall (line 110-111):**
```
chown root:root /etc/systemd/system
chmod 755 /etc/systemd/system    # drwxr-xr-x — correct restore
```
✓ Restore is correct.

### 3. `systemd-journal` group membership NOT removed on uninstall

**Install (line 15):**
```bash
usermod -aG systemd-journal lighthouse
```
**Uninstall:** `userdel lighthouse` — this removes the user entirely, so the secondary
group membership disappears with it. ✓ Correct — no action needed.

### 4. nginx configs: uninstall looks for `*.conf` files with comment `# managed by lighthouse`

**Install writes via `nginx/manager.go` (line 36):**
```go
header := fmt.Sprintf("# managed by lighthouse\n# app: %s\n# generated: %s\n\n", ...)
```
The comment written is `# managed by lighthouse` (lowercase, no trailing spaces).

**Uninstall (line 60):**
```bash
if grep -q "# managed by lighthouse" "$conf" 2>/dev/null; then
```
✓ Exact match — compatible.

### 5. [BUG] nginx `sites-enabled` symlinks may not be cleaned up if `sites-available` is already gone

**Uninstall (lines 58-66):**
```bash
for conf in /etc/nginx/sites-available/*.conf; do
    if grep -q "# managed by lighthouse" "$conf"; then
        rm -f "/etc/nginx/sites-enabled/$name"
        rm -f "$conf"
    fi
done
```
The loop globs only `sites-available`. If a dangling symlink exists in `sites-enabled`
pointing to a file that was already manually deleted from `sites-available`, the loop
never touches it. After `rm -rf /etc/lighthouse` (step 5), cert paths referenced in
those orphan symlinks are also gone — `nginx -t` (line 67) would fail but the `|| true`
swallows the error. **Orphan symlinks remain in `sites-enabled`, nginx is broken.**

**Fix:** Also glob `sites-enabled` for orphan symlinks, or remove all `sites-enabled`
symlinks whose target no longer exists.

### 6. [BUG] Ordering: `/etc/lighthouse` removed (step 5) BEFORE nginx cleanup reload (step 3/line 67)

Actually checking the order:
- Step 3 (lines 57-68): nginx cleanup and reload — OK, `/etc/lighthouse` still exists here
- Step 5 (lines 87-90): removes `/etc/lighthouse` (including certs)
- Step 7 (lines 96-99): removes `lighthouse.service` and daemon-reload

The ordering is correct: nginx cleanup happens before certs are deleted. ✓

### 7. [BUG] `/var/lib/lighthouse` removed (step 5) but app `AppDir` data under it is NOT listed in "not removed"

**Install:** apps are cloned to `/var/lib/lighthouse/apps/<name>` (from `build.go`)  
**Uninstall:** `rm -rf /var/lib/lighthouse` deletes all cloned app source trees  
**But uninstall banner (line 29) says:** `✓  Your app binaries/jars` will NOT be removed

This is misleading — the **cloned source code** is silently deleted. The built
`.jar` or binary path is stored in the DB (`binary_path` column). For Spring Boot,
`BinaryPath` points inside the app's `AppDir` (under `/var/lib/lighthouse/apps/`),
so **the jar IS deleted** too despite the banner saying otherwise.

For Go apps, `BinaryPath` is `<AppDir>/<name>` — also under `/var/lib/lighthouse/apps/`.

The banner is wrong — app source + binaries ARE deleted. This is only a UX/documentation
issue, not a functional failure, but users would lose source code they assumed was safe.

### 8. [BUG] `groupdel lighthouse` before `userdel lighthouse` — ordering issue

**Uninstall (lines 115-116):**
```bash
userdel lighthouse || true
groupdel lighthouse || true
```
`userdel` on a system user with `--system` and `--no-create-home` removes the user.
The primary group (`lighthouse`) was created as a system group. `groupdel` then removes it.
This is the correct order (user first, then primary group).

However: if `userdel` fails (e.g. a process still running), `groupdel` is still
attempted. Since the group is the lighthouse user's **primary group**, `groupdel` will
fail with "cannot remove primary group of user 'lighthouse'" if the user still exists.
The `|| true` silently swallows this, leaving an inconsistent state.

**Fix:** Check that `userdel` succeeded before calling `groupdel`, or use `|| true`
only on `groupdel` and handle `userdel` failure explicitly.

### 9. [BUG] lighthouse.service unit file path not removed before daemon-reload

**Uninstall step 2 (lines 46-54):** removes `lighthouse-*.service` files and daemon-reloads  
**Uninstall step 7 (lines 96-99):** removes `lighthouse.service` and daemon-reloads

There are TWO daemon-reloads: one after app units (step 2) and one after the main
lighthouse.service (step 7). This is fine and intentional. ✓

### 10. `/etc/lighthouse/envs` dir — not mentioned in uninstall banner but IS deleted

**Install:** creates `/etc/lighthouse/envs/`  
**Uninstall:** `rm -rf /etc/lighthouse` — deletes everything including env files  
**Banner:** only mentions "Config + certs: /etc/lighthouse/"  

Env files contain user-defined environment variables for deployed apps. These are
silently deleted with no backup offered, despite containing potentially important config.

---

## Summary Table

| # | Type | Description | Severity |
|---|------|-------------|----------|
| 5 | Bug | Orphan symlinks in `sites-enabled` not cleaned if `sites-available` already gone | Medium |
| 7 | Bug/UX | Banner claims "app binaries not removed" but they ARE (stored under `/var/lib/lighthouse/apps/`) | Medium |
| 8 | Bug | `groupdel` called even when `userdel` fails — leaves inconsistent user/group state | Low |
| 10 | UX | Env files in `/etc/lighthouse/envs/` deleted silently without backup offer | Low |
| 4 | ✓ | nginx marker comment — exact match, compatible | - |
| 6 | ✓ | Deletion ordering (certs before nginx config?) — correct | - |
| 9 | ✓ | Two daemon-reloads — intentional and correct | - |
