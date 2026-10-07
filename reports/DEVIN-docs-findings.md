# Devin CLI hooks: documentation findings (2026-10-06). No empirical fixtures yet.

Sources: the local Devin CLI docs shipped with the installed Desktop app (`...\devin\share\devin\docs\extensibility\hooks\overview.mdx`, `lifecycle-hooks.mdx`, `reference\configuration\global-vs-local.mdx`, `reference\commands.mdx`, `changelog\stable.mdx`). Installed CLI: `devin 3000.11.3`.

## What the docs say (all UNVERIFIED empirically)
- **Events:** `PreToolUse`, `PostToolUse`, `PermissionRequest`, `UserPromptSubmit`, `Stop`, `PostCompaction`, `SessionStart`, `SessionEnd`.
- **Handler types:** `command` (stdin JSON) and `prompt` (LLM). Matcher is a regex on `tool_name`, tool events only. Tool names: `exec`, `edit`, `write`, `read`, `apply_patch`, `grep`, `glob`, `webfetch`, `run_subagent`, MCP as `mcp__<server>__<tool>`.
- **Common stdin fields:** `hook_event_name`, `session_id`, `prompt_id` (absent before first prompt). `DEVIN_PROJECT_DIR` env var is set. Per changelog: `PreToolUse` carries `tool_provenance`; `Stop` carries `last_assistant_message` and `stop_hook_active`; `SessionEnd` has `reason`; `SessionStart` has `source`.
- **Event fields:** `PreToolUse`: `tool_name`, `tool_input` (e.g. `{command, shell_id}`); `PostToolUse` adds `tool_response {success, output, error}`; `PermissionRequest`: `tool_name`, `tool_input`.
- **Decisions:** exit `0` continue; exit `2` block (reason from stderr); other non-zero = logged, does not block (fail-open). JSON `decision: approve|block`, `hookSpecificOutput.additionalContext`, `updatedInput` (merged into args).
- **Locations:** project `.devin/hooks.v1.json` (the whole file is the event map, no wrapper key), `.devin/config.json` / `config.local.json` (`"hooks"` key), user `%APPDATA%\devin\config.json` (`"hooks"` key). Hooks are **collected from all sources and all run**, deduplicated by source file. Ancestor directories up to the repo root are searched. `/hooks` lists loaded hooks.
- **`.claude/` hooks are read by default** (`read_config_from.claude`), so a Wardent entry in `.claude/settings.json` would also run under Devin. Wardent must never write there (your rule), but users who already have Claude-format hooks will see them run in Devin.
- **Not documented (must be measured):** default timeout and what happens on timeout; which shell runs `command` on Windows and how quoting works; behaviour on hook crash / missing executable / invalid JSON output; whether project hooks need workspace trust; whether hook output size is limited; what `tool_provenance` contains; real payload shapes for every event.

## What I tried
1. Read the docs above. 2. `devin --help`, `devin auth status`: **"Not logged in."** (credentials would live at `%APPDATA%\devin\credentials.toml`). So `devin -p` runs cannot be made without you logging in. I did not attempt to authenticate.
3. Placed a capture-only probe at the repo-root `.devin/hooks.v1.json` and ran a shell command from this Devin Desktop session: **no hook fired** (no probe log). The Desktop agent here does not load project `.devin` hooks mid-session (or at all). I removed the probe file. Conclusion: this session cannot serve as a fixture source.

## Prepared and ready (throwaway)
`spike/devin/probe.py` (records stdin/env/parent process chain; fault injection modes `exit1, exit2, blockjson, approvejson, badjson, garbage, crash, hang, slow, bigout, exit2json` selected by a `CASE_<mode>` marker in an `exec` command) and `spike/devin/proj/` (git-initialised throwaway project with a fake `.env` and a probe `.devin/hooks.v1.json`, timeout 5 s).

## Planned Devin experiments (need `devin auth login` by you, then one or two short `devin -p` runs each)
| ID | Question |
| :- | :- |
| D1 | Real payloads for all 8 events on Windows, incl. `tool_provenance`, `tool_response`, path format, `edit`/`apply_patch` shapes |
| D2 | Which shell runs `command` (cmd / PowerShell / Git Bash); quoting of a command with quotes and spaces; parent chain |
| D3 | Failure semantics: exit 1, crash, missing exe, invalid JSON, plain text output, 2 MB stdout, hang vs `timeout: 5`, hang with no timeout (default), exit 2 with stderr, JSON block, exit 2 + approve JSON |
| D4 | Project hooks vs workspace trust; user-config hooks; `.devin` vs `.claude` duplicate firing; effect of `read_config_from.claude=false` |
| D5 | Can a project override/disable user hooks (tamper)? Is there a `disableAllHooks`-like switch? What does `/hooks` show? |
| D6 | Latency overhead per tool call with a no-op Go shim vs probe (E6 equivalent), run with the Go `wardent` binary from E5 |

## Status of your Phase 1 preconditions
| Precondition | State |
| :- | :- |
| E1/E2/E3/E7/E9 results | Not available. The Devin equivalents (D1-D6) are not run; E9 (native overlap) is not planned for Devin yet. |
| ADR-001 language | **Not written** (Go recommended; you replied "yes" to approve it, but the ADR file doesn't exist yet; I'm writing it only once the integration contract is verified, to keep ADRs consistent) |
| ADR-002 integration contract | **Not writable** without D1-D3 |
| ADR-003 go/no-go | **Not writable**; therefore **NO-GO by your rule** until D1-D3 pass |
