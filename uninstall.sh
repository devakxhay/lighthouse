#!/bin/bash
set -e

# Detect caller's home directory when running with sudo
if [ -n "$SUDO_USER" ]; then
    USER_HOME=$(getent passwd "$SUDO_USER" | cut -d: -f6)
else
    USER_HOME=$HOME
fi
BACKUP_DIR="$USER_HOME/lighthouse-ca-backup"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Lighthouse Uninstaller"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "This will remove:"
echo "  ✗  Lighthouse service (stopped + disabled)"
echo "  ✗  Binary: /usr/local/bin/lighthouse"
echo "  ✗  Helper Binary: /usr/local/bin/lighthouse-install-bins"
echo "  ✗  Config + certs: /etc/lighthouse/"
echo "  ✗  Database: /var/lib/lighthouse/"
echo "  ✗  Sudoers: /etc/sudoers.d/lighthouse"
echo "  ✗  System user + group: lighthouse"
echo "  ✗  All nginx configs written by Lighthouse"
echo "  ✗  All systemd units written by Lighthouse (lighthouse-*.service)"
echo ""
echo "The following will NOT be removed:"
echo "  ✓  nginx itself"
echo ""
echo "Note:"
echo "  !  App source code + built binaries under /var/lib/lighthouse/apps/ WILL be deleted"
echo "  !  Env files under /etc/lighthouse/envs/ WILL be deleted (back up manually if needed)"
echo "  ✓  Your CA is backed up to $BACKUP_DIR before removal"
echo ""
read -p "Are you sure you want to uninstall Lighthouse? [y/N]: " confirm

if [[ ! "$confirm" =~ ^[yY]$ ]]; then
    echo "Uninstall cancelled."
    exit 0
fi

# 1. Stop and disable Lighthouse service
if systemctl is-active --quiet lighthouse 2>/dev/null; then
    systemctl stop lighthouse || true
fi
systemctl disable lighthouse || true
echo "✓  Lighthouse service stopped"

# 2. Stop all managed app services (lighthouse-*.service)
for unit in /etc/systemd/system/lighthouse-*.service; do
    [ -e "$unit" ] || continue
    name=$(basename "$unit")
    systemctl stop "$name" || true
    systemctl disable "$name" || true
    rm -f "$unit"
    echo "✓  Removed service: $name"
done
systemctl daemon-reload
echo "✓  All managed services removed"

# 3. Remove nginx configs written by Lighthouse
for conf in /etc/nginx/sites-available/*.conf; do
    [ -e "$conf" ] || continue
    if grep -q "# managed by lighthouse" "$conf" 2>/dev/null; then
        name=$(basename "$conf")
        rm -f "/etc/nginx/sites-enabled/$name"
        rm -f "$conf"
        echo "✓  Removed nginx config: $name"
    fi
done
# Also remove any orphan symlinks in sites-enabled pointing to now-deleted files
for link in /etc/nginx/sites-enabled/*.conf; do
    [ -L "$link" ] || continue
    if [ ! -e "$link" ]; then
        rm -f "$link"
        echo "✓  Removed orphan symlink: $(basename "$link")"
    fi
done
nginx -t && systemctl reload nginx || true
echo "✓  Nginx cleaned up"

# 4. Backup CA before deleting
if [ -d /etc/lighthouse/ca ]; then
    if [ -f /etc/lighthouse/ca/ca.crt ]; then
        mkdir -p "$BACKUP_DIR"
        cp /etc/lighthouse/ca/ca.crt "$BACKUP_DIR/ca.crt"
        if [ -f /etc/lighthouse/ca/private/ca.key ]; then
            cp /etc/lighthouse/ca/private/ca.key "$BACKUP_DIR/ca.key"
        elif [ -f /etc/lighthouse/ca/ca.key ]; then
            cp /etc/lighthouse/ca/ca.key "$BACKUP_DIR/ca.key"
        fi
        if [ -n "$SUDO_USER" ]; then
            chown -R "$SUDO_USER":"$SUDO_USER" "$BACKUP_DIR"
        fi
        echo "✓  CA backed up to $BACKUP_DIR"
    fi
fi

# 5. Remove Lighthouse dirs
rm -rf /etc/lighthouse
rm -rf /var/lib/lighthouse
echo "✓  Config and data removed"

# 6. Remove binaries
rm -f /usr/local/bin/lighthouse
rm -f /usr/local/bin/lighthouse-install-bins
echo "✓  Binaries removed"

# 7. Remove systemd unit
rm -f /etc/systemd/system/lighthouse.service
systemctl daemon-reload
echo "✓  Systemd unit removed"

# 8. Remove sudoers
rm -f /etc/sudoers.d/lighthouse
echo "✓  Sudoers rule removed"

# 9. Restore nginx and systemd dir permissions to root
chown root:root /etc/nginx/sites-available
chown root:root /etc/nginx/sites-enabled
chmod 755 /etc/nginx/sites-available
chmod 755 /etc/nginx/sites-enabled
chown root:root /etc/systemd/system
chmod 755 /etc/systemd/system
echo "✓  Nginx & systemd permissions restored"

# 10. Remove lighthouse user and group
# Note: userdel automatically removes the primary group when no other users share it.
# groupdel is only needed as a safety net (e.g. group was not cleaned up by userdel).
if userdel lighthouse 2>/dev/null; then
    # Only call groupdel if the group still exists (userdel may have already removed it)
    getent group lighthouse > /dev/null && groupdel lighthouse 2>/dev/null || true
    echo "✓  System user and group removed"
else
    echo "⚠  Could not remove lighthouse user (still running?). Group not removed."
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ✓  Lighthouse uninstalled successfully."
echo ""
echo "  Your CA was backed up to $BACKUP_DIR"
echo "  Keep ca.key safe — anyone with it can sign certs trusted by your devices."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
