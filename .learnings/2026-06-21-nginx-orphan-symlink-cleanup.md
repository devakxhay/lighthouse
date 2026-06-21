# Nginx Config Test Validates ALL Sites-Enabled — Orphan Symlink Bug

**Date:** 2026-06-21

## Context
When deploying an app, Lighthouse writes a new nginx config to `sites-available/`, symlinks it into `sites-enabled/`, and runs `nginx -t` to validate.

## Problem
`nginx -t` validates **every** config file referenced in `sites-enabled/`, not just the one being deployed. If a previously deleted app left behind a broken symlink in `sites-enabled/` (pointing to a file that no longer exists in `sites-available/`), the test fails with a misleading error referencing the **stale** file, not the app currently being deployed.

Example: deploying `kron-lan` fails with:
```
open() "/etc/nginx/sites-enabled/llm-comp.conf" failed (2: No such file or directory)
```
Even though `kron-lan.conf` is perfectly valid.

## Root Cause
The `RemoveConfig` function deletes files but doesn't guarantee the symlink is removed atomically. Race conditions, permission errors, or partial failures can leave orphan symlinks.

## Solution
1. **Orphan Symlink Cleanup**: Added `cleanOrphanSymlinks()` to `Manager`. Before every `nginx -t`, scan `sites-enabled/` for symlinks whose targets no longer exist and remove them.
2. **Improved Revert Logic**: The `revert()` function now checks if the config has any prior git commits. If it's a brand-new config that failed validation, it removes both the file and symlink entirely instead of trying `git checkout HEAD` (which would fail since there's nothing to checkout).

## Key Insight
Always treat `nginx -t` as a global operation that can fail due to unrelated state. Defensive cleanup before testing is essential.
