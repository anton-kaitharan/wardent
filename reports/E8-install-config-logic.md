# E8 (config-logic part only): installer edit algorithms (RUN) and everything else BLOCKED

Code: `spike/e8/installlogic.py` + `test_installlogic.py` (throwaway Python prototype; 12 tests, all passing). Not product code.

## What was run
Pure text/JSON/TOML manipulation of the config files Wardent's installer must edit, tested against synthetic fixtures. **The hook JSON/TOML shapes come from the vendor docs; they were NOT validated against a real Claude Code or Codex** (no account; the local `claude` 2.1.31 binary cannot run `doctor` without a TTY, and I did not run any agent session).

## Findings
1. **Claude `settings.json` / Codex `hooks.json` (JSON):** parse, merge into `hooks.<Event>[]`, re-serialize preserving key order, indentation (2/4/tab), CRLF, BOM, trailing newline, compact form, and unicode. Install is idempotent; an old/renamed Wardent entry is replaced (matched by regex on `wardent* hook <agent> <event>`), user hooks and unrelated keys are untouched. Install-then-uninstall returned the **byte-identical original in all 8 style variants tested**.
2. **Refuse-don't-clobber:** invalid JSON, JSON-with-comments, trailing commas, non-object roots, `hooks` of the wrong type: installer raises and writes nothing. Consequence: if Claude's `settings.json` legitimately allows comments (not verified), those users would be refused; needs checking against the real parser.
3. **Known limitation (documented in a test):** a pre-existing empty `"hooks": {}` is removed on uninstall. Exact restoration in every case needs a backup + manifest saved at install time (also the right design for rollback).
4. **Codex `config.toml`:** never parse-and-rewrite (would destroy comments). A marker-delimited managed block is appended as text, validated by a real TOML parser (`tomllib`), and removed by marker. Verified: coexists with user-defined `[[hooks.X]]` tables (array-of-tables merge), is idempotent, handles files without a trailing newline, and **refuses** when an inline `hooks = {...}` table already exists (appending would produce invalid TOML) or when the existing file is invalid. Alternative: write a separate `~/.codex/hooks.json`; the docs say having both in the same layer is merged with a startup warning, so prefer one representation per layer.
5. **Design implication:** the installer algorithm for ADR-002 is: read -> detect style -> validate -> backup with manifest -> edit -> re-parse the result -> atomic write (temp + rename) -> verify. Prototype covers the middle steps; backup/atomic write were not prototyped.

## BLOCKED (need an agent account or real runs)
- Settings precedence and merging as actually implemented (project vs user vs local vs managed, `disableAllHooks` override, `--bare`): **BLOCKED (Claude Code)**.
- Tamper detection / heartbeat feasibility, `ConfigChange` hook: **BLOCKED (Claude Code)**.
- Codex trust-hash scope, `/hooks` review flow, project-trust behaviour: **BLOCKED (Codex; also not installed here)**.
- PATH resolution for GUI-launched agents (IDE/desktop): **BLOCKED (needs the agents)**.
