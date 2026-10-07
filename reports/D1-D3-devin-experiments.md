# D1-D3: Devin CLI Hook Empirical Verification (2026-10-07)

Status: COMPLETED on Windows 11 (build 26200) with Devin CLI `v3000.11.3 (9c803229faa4)`.
Account used: Devin Pro tier, run on free model `swe-2-medium`.
Total live runs executed: **5** (strictly adhering to the 5-run cap).

## Summary of Findings

1. **Hooks execute reliably on Windows via Git Bash**:
   - Process tree captured: `devin.exe` -> `"C:\Program Files\Git\bin\..\usr\bin\bash.exe" -c "<command>"`.
   - Command hooks receive JSON on stdin via standard pipes.
   - Environment variables passed: `DEVIN_PROJECT_DIR`, `CLAUDE_PROJECT_DIR` (pointing to project root).

2. **Hook Events Verified**:
   - `SessionStart`: payload contains `hook_event_name: "SessionStart"`, `source: "startup"`, `session_id`.
   - `UserPromptSubmit`: payload contains `hook_event_name: "UserPromptSubmit"`, `prompt`, `session_id`, `prompt_id`.
   - `PreToolUse`: payload contains `hook_event_name: "PreToolUse"`, `tool_name` (`exec`, `write`, `read`), `tool_input`, `tool_use_id`, `session_id`, `prompt_id`.
   - `PostToolUse`: payload contains `hook_event_name: "PostToolUse"`, `tool_name`, `tool_input`, `tool_response: {success: bool, output: string, error: null|string}`, `tool_use_id`, `session_id`, `prompt_id`.
   - `Stop`: payload contains `hook_event_name: "Stop"`, `stop_hook_active: bool`, `last_assistant_message: string`, `session_id`, `prompt_id`.
   - `SessionEnd`: payload contains `hook_event_name: "SessionEnd"`, `reason: "other"`, `session_id`, `prompt_id`.

3. **Blocking and Failure Semantics (Empirically Verified)**:
   - **Exit code 2 blocks**: PreToolUse hook exiting 2 aborted tool execution. The agent received the block notice and did NOT execute the command. `PostToolUse` did not fire; turn skipped directly to `Stop` and `SessionEnd`.
   - **JSON `decision: "block"` blocks**: PreToolUse hook exiting 0 with `{"decision": "block", "reason": "..."}` aborted tool execution and conveyed the reason to the agent.
   - **Exit code 1 fails open**: PreToolUse hook exiting 1 logged an error but allowed the tool call to proceed normally; `PostToolUse` fired and tool completed successfully.
   - **Observe-Only Contract for Phase 1**: Hook must exit `0` with clean stdout (or empty JSON `{}`) to guarantee zero interference with the agent loop.

4. **Fixtures Created**:
   - Stored in `fixtures/devin/3000.11.x/windows/`:
     - `session-start.json`
     - `user-prompt-submit.json`
     - `pre-tool-use-exec.json`
     - `post-tool-use-exec.json`
     - `pre-tool-use-write.json`
     - `post-tool-use-write.json`
     - `pre-tool-use-read.json`
     - `post-tool-use-read.json`
     - `stop.json`
     - `session-end.json`
