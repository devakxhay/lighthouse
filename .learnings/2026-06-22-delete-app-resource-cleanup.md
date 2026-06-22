# Delete App Resource Cleanup Pattern

## Context
When deleting a deployed application in Lighthouse, it is critical to ensure that no orphaned system resources are left behind. Leaving orphaned files or configuration state can cause conflicts when redeploying apps with the same domain or name, and wastes system resources.

## Architectural Learning
To clean up a deployed application fully, the following subsystems must be cleaned up in a specific sequence:
1. **Application Daemon (systemd)**: The application service must first be stopped, disabled, and the unit service file deleted. A daemon-reload is then triggered.
2. **Reverse Proxy (Nginx)**: The configuration file (`sites-available` and `sites-enabled` symlink) must be deleted, and a git commit representing the state change is registered. Nginx must then reload.
3. **SSL Certificates**: The SSL certificate must be revoked from the local CA database (`openssl ca -revoke`), the CRL regenerated, and the cert (`.crt`) and key (`.key`) files removed from disk.
4. **Local DNS (dnsmasq)**: The app database record is deleted first, then the dnsmasq hosts config is synchronized using the active list of applications (excluding the deleted app).
5. **Database Entry**: Deleting the application row triggers a database cascade on foreign keys, clearing the `certs` row automatically.

This ensures a clean, deterministic state across all system services.
