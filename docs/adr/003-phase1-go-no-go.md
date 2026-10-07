# ADR-003: Phase 1 (Observe and Audit) Go/No-Go Decision

Status: ACCEPTED
Date: 2026-10-07
Deciders: Devin & Antony K C

## Decision: GO

All required preconditions for Phase 1 (Observe and Audit) are satisfied:

1. **Language Choice**:
   - Evaluated in E5 (`reports/E5-coldstart.md`) and E10 (`reports/E10-shellparse.md`).
   - Go passed all warm latency gates (p95: 26–46ms vs budget 80ms; binary size: 2.5MB; zero runtime dependencies).
   - Documented in `docs/adr/001-language.md`.

2. **Integration Contract & Empirical Validation**:
   - Devin CLI hook behavior verified live on Windows 11 (`reports/D1-D3-devin-experiments.md`).
   - Verified that `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`, and `SessionEnd` fire with structured JSON.
   - Verified that `exit 0` allows the agent loop to continue unobstructed.
   - Clean fixtures scrubbed and stored in `fixtures/devin/3000.11.x/windows/`.
   - Documented in `docs/adr/002-integration-contract.md`.

3. **Phase 1 Scope**:
   - `wardent install` / `uninstall` (project `.devin/hooks.v1.json` and user `%APPDATA%\devin\config.json`).
   - `wardent hook devin <event>` (strictly observe-only, exit 0, fail-safe).
   - Local append-only JSONL audit log with secret redaction and rotation.
   - `wardent log` and `wardent explain <id>`.
   - `wardent doctor` verification and diagnostics.
   - Automated unit, contract, and chaos test suite.

We proceed directly to Phase 1 implementation.
