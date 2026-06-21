# Architectural Design: Runtime Detection and User Override Protection

When building self-hosted platforms (like Lighthouse) that orchestrate multiple languages/runtimes (Go, Node, Java/Spring Boot), handling path configuration for external dependencies requires both auto-detection and explicit manual override flexibility.

## Key Design Patterns Applied

1. **Auto-Detection with Protection Guard**
   - The auto-detection loop (`runtime.Detector`) queries executable paths via `exec.LookPath`.
   - The database upsert queries use state checks to prevent auto-detection from overwriting explicit human-entered paths:
     ```go
     if existingOverridden && !overridden {
         return nil // Skip updating path
     }
     ```

2. **Template Decoupling**
   - Systemd units dynamically receive absolute binary paths computed from database configuration (injected via `templates.AppData`). This removes runtime environment path dependency issues from systemd service processes.

3. **Pre-flight Deploy Guards**
   - Before executing build/pull sequences, a check validates if the required binary path for the specific `AppType` (e.g., Spring Boot -> `java`, Next.js -> `npm`) is available, short-circuiting failing builds early with descriptive instructions.
