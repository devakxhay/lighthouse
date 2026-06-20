# SQLite Migration Compatibility and ID Types

## Problem
The initial migration script (`0001_init.sql`) used PostgreSQL-specific features that are incompatible with SQLite:
1. `UUID` datatype.
2. `DEFAULT gen_random_uuid()` default value function.

Since SQLite does not support `gen_random_uuid()` natively, insertion would fail. Furthermore, the Go models (e.g., `models.App`) represent IDs as `int64` and retrieve auto-generated IDs via `res.LastInsertId()`.

## Solution
1. Changed `UUID PRIMARY KEY DEFAULT gen_random_uuid()` to `INTEGER PRIMARY KEY AUTOINCREMENT`.
2. Changed the foreign key referencing column `app_id` in `certs` table from `UUID` to `INTEGER`.
3. Deleted the existing `/var/lib/lighthouse/lighthouse.db` and rebuilt the application via `./install.sh` to allow the embedded migration to apply successfully.
