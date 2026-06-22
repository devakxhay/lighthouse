# Optimization of SSL Certificate Generation in Deployments

To optimize deployments, we avoid regenerating SSL certificates if they are already generated, stored in the database, present on disk, and still valid (verified via `openssl verify`).

## Implementation Details

We modified `internal/api/deploy.go` to perform a pre-check:
1. Retrieve the certificate metadata from SQLite.
2. Check if the certificate domain matches the application's domain.
3. Check if the certificate has not expired.
4. Verify that both the `.crt` and `.key` files exist on disk.
5. Use `h.SSL.Verify(certPath)` to ensure the certificate parses successfully and verifies against the configured CA certificate.

If all checks pass, we skip calling the slow `h.SSL.Generate` (which revokes and regenerates a private key/CSR/certificate) and skip updating the DB.
