# Transitioning Local Domain Suffix from .local to .internal

To avoid conflicts with multicast DNS (mDNS) / Zeroconf systems which natively reserve and use the `.local` top-level domain, we transitioned the application's default local domain suffix to `.internal`.

## Rationale

- **mDNS Conflicts**: Operating systems and local network devices query multicast DNS for `.local` hosts. Running a private DNS resolver (like `dnsmasq`) for `.local` domains can conflict with system-level mDNS routing, causing inconsistent resolution delays or failures.
- **IANA Standards**: The `.internal` TLD is globally reserved for private, local network uses, making it the correct modern choice for virtual local development routing.

## Implementation Changes

1. **DNS Setup**:
   - Modified `Sync` in `internal/dns/manager.go` to register `lighthouse.internal` for the control panel.
   - Updated `install.sh` to populate the initial `dnsmasq.conf` with `lighthouse.internal`.

2. **Nginx & SSL Configuration**:
   - Updated the SSL cert generation script within `install.sh` to sign certificates for `lighthouse.internal`.
   - Updated Nginx configurations and symlinks to target `lighthouse.internal`.

3. **Frontend UI**:
   - Replaced placeholder values in `create_modal.html` to suggest `.internal` (e.g. `et-backend.internal`) by default.
