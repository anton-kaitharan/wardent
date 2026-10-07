# Wardent Phase 1 (Observe and Audit) Final Report

**Date**: 2026-10-07  
**Platform**: Windows 11 Home (Build 26200), amd64  
**Target Agent**: Devin CLI `v3000.11.3 (9c803229faa4)`  
**Binary Built**: `wardent.exe` (Go 1.26.8, single static binary, 2.97 MB)

---

## 1. What Works

1. **`wardent install` & `uninstall`**:
   - **Project mode**: Writes hooks directly to `.devin/hooks.v1.json` at project root. Idempotent: re-running does not duplicate entries. Backs up existing file before editing.
   - **User mode (`--user`)**: Writes to `%APPDATA%\devin\config.json` under `"hooks"` key, preserving all other user settings.
   - **Safety guarantee**: Never creates or modifies `.claude/` files.
   - **`uninstall`**: Selectively strips only Wardent hooks (`wardent hook devin ...`) while leaving custom user hooks untouched. Removes empty `.devin/hooks.v1.json` if no other hooks remain.

2. **`wardent hook devin <event>`**:
   - Responds to `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`, and `SessionEnd`.
   - **Strict Observe-Only**: Always exits `0` with clean/empty stdout. Never interferes with or delays the agent loop.
   - **Fail-Safe Recovery**: Catches panics, malformed inputs, truncated payloads, binary garbage, and disk write failures; logs diagnostics to `wardent_diagnostics.log` and still exits `0`.
   - **Deadline Enforcement**: Internal 1500ms timeout prevents hangs even if stdin pipe or disk stalls.

3. **Audit Log & Redaction**:
   - Append-only JSONL written to `%LOCALAPPDATA%\wardent\audit.jsonl` (or `XDG_DATA_HOME` / `WARDENT_DATA_DIR`).
   - Redacts sensitive credentials **before** writing to disk:
     - AWS keys (`AKIA...`), GitHub tokens (`ghp_...`, `github_pat_...`), OpenAI/Anthropic keys (`sk-...`), Slack tokens (`xoxb-...`), Stripe keys (`sk_live_...`).
     - Multiline RSA/EC private keys (`-----BEGIN PRIVATE KEY-----`).
     - Authorization headers (`Bearer ...`, `Authorization: ...`).
     - `.env` variable assignments and script parameters (`PASSWORD=...`, `SECRET=...`).
   - Normalizes Windows backslashes (`\`) to forward slashes (`/`) in canonical event paths.
   - Rotates automatically by size (default: 10MB, keeping up to 5 backups) and cleans files older than 30 days.

4. **`wardent log` & `wardent explain`**:
   - Formatted human-readable audit trail with timestamp, agent, event kind, tool, and command/file details.
   - Filterable by `--session <id>`, `--tool <name>`, and `--limit <n>`.
   - `wardent explain <id>` outputs structured payload inspection with advisory security notices (previewing Phase 2 rules).

5. **`wardent doctor`**:
   - Verifies binary availability on system `PATH`.
   - Verifies project and user hook configuration integrity.
   - Executes live in-memory hook round-trip.
   - Reads `heartbeat.json` to verify live agent event arrivals.
   - Detects and warns about conflicting legacy directory configurations.

---

## 2. Measured Latency (Benchmark vs E5 Budget)

Tested via `BenchmarkHookLatency` on Intel Core i5-8300H CPU @ 2.30GHz (Windows 11 with Windows Defender active):

| Metric | Measured | E5 Target Budget | Result |
| :--- | :--- | :--- | :--- |
| **p50 (median)** | **18.98 ms** | < 40 ms | **PASS** |
| **p95** | **43.98 ms** | <= 80 ms | **PASS** |
| **p99** | **72.57 ms** | <= 150 ms | **PASS** |

*Note*: Execution time includes full JSON parsing, schema normalization, path conversion, secret pattern scanning, JSONL log formatting, and file I/O flush.

---

## 3. What Does Not Work / Out of Scope (Phase 1)

As specified in Phase 1 constraints:
- **No blocking or asking**: Wardent never blocks actions or returns decision prompts in Phase 1.
- **No policy engine**: Per-repo `policy.toml` rules and thresholds begin in Phase 2 (Risk Guard).
- **No model / Laya**: Semantic evaluation is deferred to Phase 4.
- **No loop detection or result verification**: Deferred to Phase 3.
- **No network calls or telemetry**: 100% offline and local.

---

## 4. Known Gaps & UNVERIFIED Items

1. **Non-Interactive Hook Shell on Windows**:
   - Devin CLI executes hooks via `"C:\Program Files\Git\bin\..\usr\bin\bash.exe" -c "<command>"`. If Git Bash is not installed on a Windows machine, hook execution behavior is **UNVERIFIED**.
2. **PermissionRequest Hook**:
   - While supported in the adapter schema, Devin CLI in `dangerous` / `auto` mode did not emit `PermissionRequest` events in our test runs (it fires only during interactive prompts).
3. **macOS & Linux Live Execution**:
   - Unit and chaos tests pass cross-platform, but live Devin CLI execution was performed only on Windows 11. Live behavior on macOS and Linux remains **UNVERIFIED**.
4. **Devin Smart Mode Classifier Overlap**:
   - Devin's built-in Smart Mode model blocks some categories of risky commands natively. The exact empirical overlap on a large test set (E9 equivalent) is **UNVERIFIED**.

---

## 5. Manual Test Checklist for Windows

To test Wardent live on your machine:

1. **Check System Health**:
   ```powershell
   .\wardent.exe doctor
   ```
   *Expected*: Shows green checks `[✓]` for CLI on PATH (if current directory is in PATH), User Global Config, Live Hook Round-Trip, and Heartbeat.

2. **Install Hooks in a Test Repo**:
   ```powershell
   mkdir test-repo
   cd test-repo
   ..\wardent.exe install
   ```
   *Expected*: Prints success message. Inspect `.devin\hooks.v1.json` to verify all 6 hook events are registered.

3. **Run a Devin CLI Task**:
   ```powershell
   devin -p "echo 'testing wardent observe mode'" --model swe-2-medium --respect-workspace-trust false
   ```
   *Expected*: Devin CLI executes without interruption or prompt.

4. **Verify Audit Trail**:
   ```powershell
   ..\wardent.exe log
   ```
   *Expected*: Displays table showing `devin`, `tool_pre`, `tool_post`, and `echo 'testing wardent observe mode'`.

5. **Explain an Event**:
   ```powershell
   # Copy the record ID from the log or audit file
   ..\wardent.exe explain <record_id>
   ```
   *Expected*: Displays detailed inspection showing `Mode: OBSERVE-ONLY (Phase 1)` and `Action: allow_silent`.

6. **Clean Up / Uninstall**:
   ```powershell
   ..\wardent.exe uninstall
   cd ..
   Remove-Item -Recurse -Force test-repo
   ```
   *Expected*: Removes `.devin\hooks.v1.json` cleanly.
