# UI Status Badges and Micro-Animations

**Date:** 2026-06-21

## Context
Applications managed by Lighthouse go through several lifecycles, transitioning through statuses like `pending`, `building`, `running`, `stopped`, and `failed`.

## Problem
While the backend processes and DB stored `building` and `pending` states, these statuses were not correctly mapped to `live_status` in the list API `/api/apps`, nor were they fully supported with appropriate CSS classes and visual indications in the frontend UI. The building indicator lacked visual engagement to indicate progress.

## Solution
1. **API Mapping**: Updated `ListApps` in [apps.go](file:///home/akxhd/lighthouse/internal/api/apps.go) to explicitly map database statuses `building` and `pending` to `live_status` so the frontend receives the correct state.
2. **UI Class Mapping**: Updated `statusClass` in [app.js](file:///home/akxhd/lighthouse/ui/app.js) to return `badge-building` when the status is `building`.
3. **Micro-Animation Styling**: Added styling for `.badge-building` in [style.css](file:///home/akxhd/lighthouse/ui/style.css) using a CSS `@keyframes` pulse animation on the badge indicator dot to visually denote build progress.

## Key Insight
UI state indicators should align perfectly with backend data models. Adding micro-animations like pulse patterns to active states (e.g. building or deployment progress) creates a premium and interactive user experience.
