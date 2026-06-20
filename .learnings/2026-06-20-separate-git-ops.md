# Separate Git-backed Configuration Actions from Core Nginx Actions

**Date:** 2026-06-20

## Context
Nginx configurations are versioned using a Git repository inside the `sites-available` directory. Originally, all file operations, Nginx commands, and Git commits were in the single file [nginx.go](file:///d:/ak/lighthouse/internal/nginx/nginx.go).

## Learning / Architectural Decision
Grouping all versioning logic (Git init, commit, checkout, log parsing) together with core Nginx command administration makes the file harder to read, maintain, and unit test. 

To improve maintainability:
1. We separated Git actions and logs parsing into [git.go](file:///d:/ak/lighthouse/internal/nginx/git.go).
2. We kept Nginx-specific logic (writing files, testing syntax, reloading service, symlinking) inside [nginx.go](file:///d:/ak/lighthouse/internal/nginx/nginx.go).
3. Both files share the package-level visibility of methods on `Manager` (e.g., `gitRun`, `revert`, `run`) keeping the package public API clean and unchanged while separating the implementation concerns.
