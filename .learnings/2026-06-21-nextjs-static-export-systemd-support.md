# Next.js Static Export (output: export) Service Execution

**Date:** 2026-06-21

## Context
When a Next.js application is configured with static HTML export (`output: 'export'`), building the app produces static files in the `out/` directory instead of a dynamic server bundle.

## Problem
Lighthouse generated a standard systemd service unit for Next.js apps that defaults to running `npm run start` (which executes `next start` underneath). However, running `next start` on a static export build crashes immediately:
```
Error: "next start" does not work with "output: export" configuration. Use "npx serve@latest out" instead.
```
This causes the systemd service to enter a crash-loop / failure state.

## Solution
1. **Dynamic Config Detection**: Implemented `isNextJSExport` in the deployment flow. This helper function:
   - Scans `next.config.js`, `next.config.mjs`, or `next.config.ts` for the `output: 'export'` option.
   - Fallback-checks if the build has populated the `out/` directory.
2. **Dynamic ExecStart**: Updated the `0002_next_js.service` template:
   - If the application is detected as static export, start the lightweight `serve` static server non-interactively using `npx --yes serve -l {{.Port}} out`.
   - Otherwise, fallback to the standard `{{.NpmBin}} run start`.

## Key Insight
Applications built on node or dynamic frameworks can compile to static targets. System managers must detect target output configurations at deploy time and dynamically swap runtimes/servers to avoid runtime startup crashes.
