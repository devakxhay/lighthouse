# Self-Registration of Lighthouse itself on Nginx and DNS

To enable a seamless local-first experience where Lighthouse is accessible securely over HTTPS at `https://lighthouse.local`, we implement self-registration during installation and clean cleanup during uninstallation.

## Self-Registration (Install)

1. **Certificate Generation**:
   - During `install.sh`, after initializing the Root CA, we generate a private key and sign a domain certificate specifically for `lighthouse.local` using the generated root CA (configured with a SAN extension for `lighthouse.local` and `*.lighthouse.local`).

2. **Nginx Setup**:
   - We write `/etc/nginx/sites-available/lighthouse.conf` with a proxy pass to Lighthouse's server port (9000).
   - This block includes `# managed by lighthouse` comment.
   - We symlink this config to `sites-enabled/lighthouse.conf` and reload Nginx.

3. **DNSmasq Setup**:
   - We write `address=/lighthouse.local/<local-ip>` to `/etc/lighthouse/dnsmasq.conf`.
   - In Go (`internal/dns/manager.go`), we update the `Sync` method to always append `address=/lighthouse.local/<local-ip>` on every DNS synchronization, ensuring the DNS record for Lighthouse itself is never lost when modifying other applications.
   - We restart the `dnsmasq` service.

## Cleanup (Uninstall)

1. **Nginx Config Removal**:
   - The uninstaller (`uninstall.sh`) automatically matches all configs containing `# managed by lighthouse`, thereby deleting `sites-available/lighthouse.conf` and its enabled symlink, and reloading Nginx.

2. **DNSmasq Cleanup**:
   - We explicitly delete `/etc/dnsmasq.d/lighthouse.conf` (the symlink to Lighthouse's DNS rules).
   - We restart `dnsmasq` so it clears the configuration.
