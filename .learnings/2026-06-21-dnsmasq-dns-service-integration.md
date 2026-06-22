# local DNS Management with dnsmasq integration

**Date:** 2026-06-21

## Context
Lighthouse functions as a deployment manager for local server environments. While Nginx handles routing requests for domains to their respective backend services, client machines on the local network need DNS resolution to direct those domain names to the Lighthouse server's IP address.

## Problem
Lighthouse lacked a system/service to automatically configure local DNS resolution. When applications were registered and deployed, users had to configure host files or local DNS servers manually.

## Solution
1. **DNS Manager Service**: Developed a new `dns.Manager` in [manager.go](file:///home/akxhd/lighthouse/internal/dns/manager.go) which:
   - Detects the host's primary local network IP address (excluding loopbacks).
   - Generates a `dnsmasq` compatible configuration listing all deployed app domains (e.g. `address=/domain/ip`).
   - Restarts the `dnsmasq` systemd service using `sudo systemctl restart dnsmasq`.
2. **Dynamic Sync Execution**: Integrated DNS syncing into the deployment ([deploy.go](file:///home/akxhd/lighthouse/internal/api/deploy.go)) and deletion ([apps.go](file:///home/akxhd/lighthouse/internal/api/apps.go)) flows.
3. **Privilege and Setup Isolation**: Modified [install.sh](file:///home/akxhd/lighthouse/install.sh):
   - Created the `/etc/lighthouse/dnsmasq.conf` file owned by the `lighthouse` user so the manager can write configuration updates without elevated privileges.
   - Symlinked the writable configuration into `/etc/dnsmasq.d/lighthouse.conf` so dnsmasq naturally incorporates it.
   - Added passwordless sudo rules allowing `systemctl restart dnsmasq` to reload the daemon.

## Key Insight
When creating services that edit root-owned daemon configurations (like dnsmasq or nginx), keep the application logic simple and secure by writing to user-owned configurations and bridging them to system paths via symlinks configured during installation. This keeps the application daemon running with minimal privileges.
