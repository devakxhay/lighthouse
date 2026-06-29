# Node Runtime installation missing npx symlink and DB registration

## Problem
During deployment of apps requiring `npx` (e.g. Marp), the `npx` executable was not found. This was due to:
1. `npx` not being registered or checked as a startup runtime target.
2. The Node.js binary installer (`internal/bins/node.go`) only symlinking and saving DB overrides for `node` and `npm`, completely omitting `npx`.

## Solution
1. Added `npx` to the list of startup-detected runtimes in `internal/runtime/detect.go`.
2. Updated the Node installer in `internal/bins/node.go` to look for `/usr/local/node/bin/npx`, symlink it to `/usr/local/bin/npx`, and register the override path in the database.
3. Configured `internal/api/build.go` to retrieve the database-registered path for `npx` during building.
