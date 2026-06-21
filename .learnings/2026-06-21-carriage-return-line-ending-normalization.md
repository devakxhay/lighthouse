# Carriage Return Line Ending Normalization in Deployed Configurations

**Date:** 2026-06-21

## Context
When running on different development platforms (like Windows vs Linux), git checking out files can sometimes convert line endings to CRLF (`\r\n`). In Go, when templates are embedded (`//go:embed`) or read, they retain these carriage returns (`\r`).

## Problem
When templates with CRLF line endings are parsed and rendered, the output retains the `\r` character. For UNIX configuration engines like Nginx, these characters are interpreted literally as `^M` line ending characters, causing syntax errors, parsing issues, or warning flags in configuration tests.

## Solution & Architectural Learning
To make the template rendering pipeline completely immune to development environment OS variations:
1. **Source Sanitization**: Strip all `\r` characters from the source template string before sending it to the template engine parser.
2. **Output Sanitization**: Run a second pass to strip any carriage returns that might have been introduced during rendering or formatting:
   ```go
   strings.ReplaceAll(renderedText, "\r", "")
   ```
This ensures all output files written to disk (e.g. systemd service units, Nginx server blocks) are normalized to pure UNIX line endings (LF).
