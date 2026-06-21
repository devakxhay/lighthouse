# Custom Go Entry Point Support

## Context
Lighthouse previously built Go applications by looking for a hardcoded `main.go` file at the root of the app directory. For applications where the entry point is located in a subdirectory (e.g. `cmd/main.go` or `cmd/app/main.go`), builds failed.

## Implementation Details
To support optional custom entry points, we extended the application registration and build system end-to-end:

1. **Database Schema & Migrations**:
   - Added a new migration `0005_entry_point.sql` containing: `ALTER TABLE apps ADD COLUMN entry_point TEXT NOT NULL DEFAULT '';`.
   - Updated the embedded migrations list and db methods `CreateApp`, `GetApp`, and `ListApps` in [sqlite.go](file:///home/akxhd/lighthouse/internal/db/sqlite.go).

2. **Model**:
   - Added `EntryPoint` string to `models.App` in [models.go](file:///home/akxhd/lighthouse/models/models.go).

3. **Backend API & Build**:
   - Bound the `entry_point` JSON field in [api.go](file:///home/akxhd/lighthouse/internal/api/api.go).
   - In [build.go](file:///home/akxhd/lighthouse/internal/api/build.go), the Go build step was updated to construct the path relative to the command's working directory (`app.AppDir`). If `app.EntryPoint` is not set, it defaults to `main.go`.

4. **Frontend UI**:
   - Updated the modal state in [app.js](file:///home/akxhd/lighthouse/ui/app.js) to initialize and clean the `entry_point` attribute.
   - Added a conditional form group to [create_modal.html](file:///home/akxhd/lighthouse/ui/components/create_modal.html) displaying the entry point input field only when the app type is `go`.
