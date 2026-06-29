# Prevent CA Cert Serial and CRL Number Reset on Upgrade

## Context
When running `install.sh` to reinstall or upgrade Lighthouse, the certificate serial number and CRL number files under `$CA_DIR/serial` and `$CA_DIR/crlnumber` were being unconditionally overwritten with `1000`. This reset the serial counter, which could cause duplicate certificate serial numbers and conflicts.

## Fix
Modified `install.sh` to check if `$CA_DIR/serial` and `$CA_DIR/crlnumber` already exist before writing the default starting value of `1000`. This preserves the existing counters across upgrades and reinstalls.
