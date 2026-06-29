# Registering Existing Running Services

## Context
A request was made to support registering and deploying existing running services within the Lighthouse dashboard. These services are run and managed externally, so Lighthouse only provides Nginx reverse proxying, SSL certificate generation, and DNS entries.

## Architectural Decision
1. **AppType Constants**: Added a new application type `service` (value `"service"`).
2. **Selective Deployment Steps**: Updated the deployment flow to skip code checkout, build, systemd service unit generation/registration, and unit lifecycle restarts for the `service` type.
3. **External Port Connectivity Verification**: The service status check utilizes TCP dial testing to the configured port on localhost to determine if the service is live, since systemd unit states do not apply to external services.
4. **Validation and UI Adjustments**: Hides build-related fields (Git URL, App Directory, Go Entry Point) when the user registers an existing service, requiring only the name and port, and defaulting the domain to `<name>.internal`.
