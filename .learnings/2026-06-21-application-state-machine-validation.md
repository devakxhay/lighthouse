# Application State Machine Validation and Blocking Rules

**Date:** 2026-06-21

## Context
In a deploy manager framework, applications go through various lifecycle stages: registration/creation (pending), building/deployment (building), running (running), and stopped (stopped).

## Problem
Allowing concurrent operations (like multiple deploys or calling Start/Stop/Restart actions on an app while it is still building or has not yet been deployed) can lead to race conditions, overlapping processes, or systemd errors because unit files are not yet fully generated.

## Solution & Architectural Learning
To enforce robust state validation:
1. **Added `StatusBuilding` status**: Defined a first-class `StatusBuilding = "building"` state to track active compilation/building.
2. **State Validation Guards**: 
   - **Deploy**: Block new deployments if current status is `StatusBuilding`. Set status to `StatusBuilding` at start, to `StatusFailed` if build fails, and `StatusRunning` when deployment successfully completes.
   - **Start**: Block start commands if the app is currently `StatusBuilding` (preventing conflicts), `StatusPending` (must deploy first), or `StatusRunning` (no-op).
   - **Stop**: Block stop commands if the app is currently `StatusBuilding` (preventing conflicts), `StatusPending` (must deploy first), or `StatusStopped` (no-op).
   - **Restart**: Block restart commands if the app is currently `StatusBuilding` (preventing conflicts) or `StatusPending` (must deploy first).

This guarantees that application actions are only allowed when the application is in a stable, ready state.
