# ADR-002: Integration Contract for Devin CLI (Phase 1 Observe & Audit)

Status: ACCEPTED
Date: 2026-10-07
Deciders: Devin & Antony K C

## Context
Wardent Phase 1 provides an observe-and-audit safety layer for coding agents. Devin CLI is the first-class target for release 1. Based on empirical experiments D1–D3 (verified on Windows 11 with Devin CLI `3000.11.3`), we define the exact integration contract.

## Contract Decisions

1. **Hook File Locations**:
   - **Project-level**: `.devin/hooks.v1.json` at the project root. The top-level JSON is the event map itself (no wrapper key).
   - **User-level (`--user`)**: `%APPDATA%\devin\config.json` on Windows (`~/.config/devin/config.json` on Unix), nested under the `"hooks"` key.
   - Wardent will **never write or modify `.claude/` files**.

2. **Hook Invocations & Command Strings**:
   - A single, stable CLI command format is used:
     `wardent hook devin <event-slug>`
   - Event slugs:
     - `SessionStart` -> `session-start`
     - `UserPromptSubmit` -> `user-prompt-submit`
     - `PreToolUse` -> `pre-tool-use`
     - `PostToolUse` -> `post-tool-use`
     - `Stop` -> `stop`
     - `SessionEnd` -> `session-end`
   - Example entry in `.devin/hooks.v1.json`:
     ```json
     {
       "PreToolUse": [
         {
           "matcher": ".*",
           "hooks": [
             {
               "type": "command",
               "command": "wardent hook devin pre-tool-use",
               "timeout": 5
             }
           ]
         }
       ]
     }
     ```

3. **Observe-Only Guarantees (Phase 1)**:
   - Always exit `0`.
   - Never print blocking decision JSON or exit with code 2.
   - Any internal panic, parsing failure, disk write error, or timeout must be caught internally, recorded in a local diagnostic log, and still exit `0` cleanly.

4. **Execution Shell & Timeouts**:
   - On Windows, Devin CLI invokes command hooks via Git Bash (`bash.exe -c "<command>"`).
   - Wardent enforces an internal execution deadline (e.g. 1.5 seconds) well below the configured hook timeout (5 seconds).

5. **Payload Parsing to AgentEvent**:
   - Incoming stdin JSON is mapped into the standard `AgentEvent` schema:
     - `hook_event_name` -> normalized `event_type`
     - `session_id` -> `session_id`
     - `prompt_id` -> `turn_id`
     - `tool_name` -> `tool.name` (normalized into `shell` for `exec`, `file_write` for `write`/`edit`, `file_read` for `read`)
     - `tool_input` -> arguments and targets
     - `tool_response` -> result status and output excerpt
     - Windows backslashes are normalized to forward slashes in canonical event representation.
