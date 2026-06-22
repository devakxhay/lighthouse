# CA Certificate Distribution and Setup Instructions

To allow clients to securely trust applications hosted on a local domain (such as `.local` or other local development domains) under HTTPS, we need to distribute the local Root CA certificate to developers' browsers and systems.

## Solution Architecture

1. **API Endpoints**:
   - `GET /api/certs/ca`: Serves the `ca.crt` file as a download using:
     - `Content-Disposition: attachment; filename=ca.crt`
     - `Content-Type: application/x-x509-ca-cert`
   - `GET /api/certs/ca/status`: Returns JSON indicating whether the CA cert exists on disk:
     - `{"exists": true|false}`

2. **Frontend UX**:
   - Added a `Trust CA` button in the topbar of the web UI.
   - Triggers an asynchronous check to `GET /api/certs/ca/status` before opening the modal.
   - If the CA cert is not available (i.e. first-time setup or clean install):
     - Disables/hides the download link and instructions.
     - Displays a warning alert instructing the user to run `sudo ./install.sh` to initialize the CA.
     - Provides a "Refresh Status" button.
   - If the CA cert is available:
     - Shows the "Download CA Certificate" button.
     - Displays tabbed instructions for macOS, Windows, Linux, Mobile, and Firefox.
     - Includes clipboard copy buttons for command-line helpers.
