# SQLite Unique Constraints and Multi-File Go Migrations

## Context
We needed to add a unique constraint on the `domain` column of the `certs` table. In SQLite, directly altering a table to add constraints after creation is restricted compared to other database systems.

## Solution
1. **Unique Index**: Instead of running a complex table recreation migration, we added a unique index:
   ```sql
   CREATE UNIQUE INDEX IF NOT EXISTS idx_certs_domain ON certs(domain);
   ```
   This effectively enforces the unique constraint on the `domain` column.
2. **Multi-File Migrations in Go**: We updated the `migrate` function in `internal/db/sqlite.go` to support executing multiple migration files in a defined sequence:
   ```go
   migrations := []string{
       "migrations/0001_init.sql",
       "migrations/0002_unique_domain.sql",
   }
   for _, m := range migrations {
       // load and Exec...
   }
   ```
