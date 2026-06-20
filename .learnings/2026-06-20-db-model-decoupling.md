# Decouple Domain Models from Database Implementation

**Date:** 2026-06-20

## Context
Initially, domain models (`App`, `Cert`, `AuditLog`, etc.) and database initialization/queries (using `go-sqlite3`) were both located under the `internal/db` package. 

## Learning / Architectural Decision
Moving domain models directly into `internal/db` couples generic domain structs to specific database implementation details. If other packages (e.g., HTTP server, runner services) need to pass or reference these models, they would be forced to import `internal/db`. This can lead to circular dependencies and leaks db details to unrelated modules.

We extracted the models into a clean, top-level [models](file:///d:/ak/lighthouse/models/models.go) package which:
1. Has zero dependencies (only standard library).
2. Can be imported freely by any package without risk of circular references.
3. Keeps [sqlite.go](file:///d:/ak/lighthouse/internal/db/sqlite.go) focused solely on DB initialization, driver imports, and query executions.
