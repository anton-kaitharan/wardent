# Wardent: Design Document (v0.2, pre-implementation)

Status: APPROVED WITH CHANGES (v0.2). No code has been written.
Research date: 2026-10-01. Integration claims below come from the official docs fetched that day (sources in section 1.5). Anything I did not verify is marked **[UNVERIFIED]**.

**Changes in v0.2 (from approval review):**
- First release = **Phases 0 to 2 only** (spike, observe/audit, Risk Guard + policy). Phases 3 to 6 remain a later roadmap (section 9).
- Phase 0 must decide Go vs TypeScript **from measured numbers** (see `docs/PHASE0_SPIKE_PLAN.md`).
- Added section 11: competitive differentiation vs native Claude Code and Codex permissions/sandboxing.
- Read the previously unread pages (section 1.5) and folded in the findings (sections 1.1, 1.2, 8).
- Resolved decisions: first buyer = individual developers (self-serve, observe-first); default failure stance = fail-safe prompting, deny only for critical hard rules, strict fail-closed as an opt-in mode (section 5, policy `[mode].failure_stance`).
- **Laya details were NOT supplied** (the answer was an unfilled template placeholder). Laya is treated as unknown; nothing in release 1 depends on it. Still open (section 10).

**Changes in v0.3 (2026-10-07, owner decision):** **Devin CLI is the only release-1 target** (first and only adapter for Phases 0 to 2). Claude Code and Codex material in sections 1.1 to 1.6, 2.4, 4 and 11 is retained as *reference for future adapters*; it is documentation-derived and **UNVERIFIED** (no account/access). Devin facts below come from the Devin CLI docs shipped with the installed app and are also UNVERIFIED empirically until experiments D1 to D6 (`reports/DEVIN-docs-findings.md`) run. The Devin CLI is currently not logged in on the dev machine, which blocks those experiments.

---

## 0. Summary of the decisions that matter

1. **Integrate through hooks, not wrappers or proxies.** Both Claude Code and Codex now ship a hooks system with a very similar shape (`PreToolUse`, `UserPromptSubmit`, `Stop`, `PostToolUse`, `PermissionRequest`, and others). One `wardent hook <agent> <event>` entrypoint, with thin per-agent adapters, covers both.
2. **Hooks are a guardrail, not a security boundary.** Both vendors say or imply this, and the failure semantics are fail-open in many cases (section 1.3). The product must be honest about this. The pitch is "catch and explain mistakes and risky behaviour", not "sandbox".
3. **The Model Router cannot be a transparent interceptor.** I found no hook that can change the model or effort. `PreModelSwitch` (Claude) can only block a switch the user requested. The Router is therefore advisory (message to the user) plus an optional launcher (`wardent run claude ...`).
4. **Wardent should almost never emit `allow`.** In Claude Code, deny and ask rules are still evaluated after a hook says allow, and an allow from us would silently bypass the user's own permission flow. Wardent emits `deny`, `ask`, `abstain` (no opinion), or context. `allow` only appears if a repo policy explicitly opts in.
5. **Codex `PreToolUse` cannot "ask"**: `permissionDecision: "ask"` is parsed but unsupported, and the hook is marked failed while the tool call continues. Our "ask the user" fail-safe therefore maps differently per agent (section 2.4).
6. **Language: I recommend challenging TypeScript for the hook path** (cold-start cost, because a process is spawned per tool call). Proposal: Go (or Rust) single static binary for CLI, hook shim, rules engine and daemon; TypeScript remains viable if you accept a daemon-first design. See section 7.

---

## 1. Integration strategy

### 1.0 Devin CLI: what is supported today (release-1 target; docs-derived, UNVERIFIED)

Source: Devin CLI docs bundled with the installed app (`extensibility/hooks/overview.mdx`, `lifecycle-hooks.mdx`, `reference/permissions.mdx`, `sandbox.mdx`, `reference/configuration/global-vs-local.mdx`); CLI `devin 3000.11.3`.

| Aspect | Devin CLI (per docs) | Implication for Wardent |
| :- | :- | :- |
| Events | `PreToolUse`, `PostToolUse`, `PermissionRequest`, `UserPromptSubmit`, `Stop`, `PostCompaction`, `SessionStart`, `SessionEnd` | Phase 1 observes `PreToolUse`, `PostToolUse`, `PermissionRequest`, `Stop`, `SessionStart` (+ `UserPromptSubmit`, `SessionEnd` if fixtures confirm) |
| Handlers | `command` (stdin JSON), `prompt` (LLM) | `command` only |
| Tool names | `exec`, `edit`, `write`, `read`, `apply_patch`, `grep`, `glob`, `webfetch`, `run_subagent`, `mcp__<server>__<tool>` | Adapter maps `exec`->shell, `edit/write/apply_patch`->file ops |
| Common fields | `hook_event_name`, `session_id`, `prompt_id`; env `DEVIN_PROJECT_DIR`; `tool_provenance` on `PreToolUse`; `last_assistant_message`/`stop_hook_active` on `Stop`; `tool_response{success,output,error}` on `PostToolUse` | Maps to `AgentEvent`; no `permission_mode`, `model`, or `cwd` documented (UNVERIFIED) |
| Block semantics | exit 2 (reason from stderr) or JSON `decision:"block"`; any other non-zero exit is logged and **does not block** (fail-open); JSON `approve` exists | Same fail-open trap as other agents; Phase 1 never blocks |
| Rewrite / context | `updatedInput` (merged into args), `additionalContext` | Not used in Phase 1 |
| Locations | project `.devin/hooks.v1.json` (whole file = event map), `.devin/config.json`/`config.local.json` and user `%APPDATA%\devin\config.json` (`"hooks"` key); **all sources collected and all run**; deduplicated by source file; ancestor dirs up to repo root | Installer writes only `.devin/hooks.v1.json` (project) or the user config `hooks` key (`--user`) |
| `.claude/` hooks | **Loaded by default** (`read_config_from.claude`) | Wardent never writes `.claude/`; a Claude-format Wardent hook would also fire in Devin |
| Not documented | default timeout and timeout outcome; Windows hook shell and quoting; behaviour on crash/missing exe/bad JSON; workspace-trust gating of project hooks; whether a project can disable hooks | Experiments D2 to D5 |
| Native permissions | modes: Normal (default), Accept Edits, Smart (fast-model judge; never auto-approves installs, mutating git, `rm`/`sudo`, cloud-destructive CLIs, dotenv/key/git-config access), Bypass, Autonomous (needs `--sandbox`); deny > ask > allow rules; org deny/ask rules via Team Settings override modes | Risk Guard overlaps Smart mode's blocklist (section 11.6) |
| Sandbox | `--sandbox` (macOS seatbelt, Linux bwrap+seccomp); **not supported on Windows; the CLI refuses to start rather than run unsandboxed** | No OS-level containment for Windows users: a deterministic guard has more value there |

---

### 1.1 Claude Code: what is supported today (reference only, UNVERIFIED, not a release-1 target)

Source: Hooks reference (https://code.claude.com/docs/en/hooks), Permissions (https://code.claude.com/docs/en/permissions).

**Hook events relevant to Wardent**

| Wardent need | Event | Can block? | Notes |
| :- | :- | :- | :- |
| Capture user intent (Scope Guard, Router) | `UserPromptSubmit` | Yes (`decision: "block"` / exit 2) | Input has `prompt`. Can inject `additionalContext`. Default timeout 30 s; timeout means prompt proceeds without our context. |
| Risk Guard, Scope Guard | `PreToolUse` | Yes | Input: `tool_name`, `tool_input`, `tool_use_id`, plus common fields. Output via `hookSpecificOutput.permissionDecision`: `allow`, `deny`, `ask`, `defer`; `updatedInput` can rewrite input. |
| Evidence ledger, Loop Detector | `PostToolUse`, `PostToolUseFailure`, `PostToolBatch` | No (stderr shown to Claude on exit 2) | `PostToolUse` Bash can carry `tool_response.bashEditDiff` (beta, best effort, "find what to review, not to enforce"). |
| Verifier ("done" claims) | `Stop` | Yes: `decision: "block"` + `reason` makes Claude continue | Input has `last_assistant_message`, `stop_hook_active`. **8-consecutive-continuation cap**, then Claude Code overrides the block. |
| Session lifecycle | `SessionStart`, `SessionEnd`, `SubagentStart/Stop`, `PreCompact` | mostly no | `SessionStart` can carry `model`. |
| Observability | `PermissionRequest`, `PermissionDenied`, `ConfigChange` | Partly | `ConfigChange` can detect someone removing our hooks. |

**Common input fields** include `session_id`, `transcript_path`, `cwd`, `permission_mode`, `hook_event_name`, `effort.level`, and `agent_id`/`agent_type` inside subagents.

**Handler types**: `command` (stdin JSON, exit code or JSON stdout), `http` (POST body = same JSON, JSON response), `mcp_tool`, `prompt`, `agent`. HTTP hooks require an allowlist (`allowedHttpHookUrls`) when defined, and "can't signal a blocking error through status codes alone". A 2xx JSON body is needed to block.

**Where hooks live**: `~/.claude/settings.json` (user), `.claude/settings.json` (project, shareable), `.claude/settings.local.json`, managed policy settings, plugins. Hook entries **merge across levels**; `allowManagedHooksOnly` exists for admins.

**Permission system** (permissions page): rules evaluate **deny, then ask, then allow**, first match wins. "Permission rules are enforced by Claude Code, not by the model." A PreToolUse hook is the documented extension point. Deny and ask rules still apply even if a hook returns `allow`.

**Fail-open traps that shape our design** (all from the hooks reference):
- Only **exit 2** blocks via exit code. Exit 1 or any other code is a non-blocking error and the action proceeds.
- A hook script that cannot start (missing path, non-executable, bad interpreter) is a non-blocking error: "a mistyped path in settings.json leaves the gate silently disabled".
- A `command`/`http`/`mcp_tool` hook that **times out does not block** `PreToolUse`. The call continues through the normal permission flow.
- Malformed JSON or schema-invalid output on exit 0 is a non-blocking error.
- HTTP connection failure or non-2xx is a non-blocking error.
- `PreToolUse` does **not** fire for `@file` references in prompts (contents are inlined by Claude Code). Only a `Read` deny rule blocks those.
- On Windows, `file_path` arrives with backslashes. Naive `/src/` matching never matches. There is also a separate `PowerShell` tool, so matchers need `Bash|PowerShell`.
- **(v0.2) Hooks can be switched off by the repo being worked on.** `disableAllHooks` is read "after settings precedence applies", so `"disableAllHooks": true` in a project's `.claude/settings.json` can override a user-level `false`. Only managed settings are immune. `claude --bare` skips auto-discovery of hooks, and `--settings '{"disableAllHooks": true}'` turns them off for a run. The docs themselves advise doing this when running `claude -p` on a repo you did not write. Wardent cannot prevent this; it must **detect** it (missing heartbeat, `doctor`, `SessionStart` self-check) and say so.
- **(v0.2) Claude Code's built-in default permission mode is now `auto`** (classifier-reviewed, v2.1.228+ on macOS/Linux/WSL, v2.1.233+ on native Windows). Its default block list already covers force-push to protected targets, `git reset --hard`, `git clean -fd`, recursive deletes on unresolved variables, and secrets pushed to public repos. See section 11 for what this means for Risk Guard.
- **(v0.2) Claude's Bash sandbox is not available on native Windows** (macOS, Linux and WSL2 only). Codex has a native Windows sandbox (`elevated`/`unelevated`).
- **(v0.2) Verified launcher flags:** `claude --model <alias|name>`, `claude --effort low|medium|high|xhigh|max|ultracode`, `CLAUDE_CODE_EFFORT_LEVEL` env var, `--settings <file|json>` (per-session override, can inject hooks without writing config), `--permission-mode`, `--setting-sources`. A launcher can therefore apply a Router recommendation for Claude Code. Effort levels available depend on the model.
- **(v0.2) PreModelSwitch** (v2.1.251+) can block a switch the user requested but cannot pick a model. Confirms the Router cannot be transparent.

### 1.2 Codex: what is supported today

Source: Codex Hooks (https://developers.openai.com/codex/hooks, `.md` variant fetched).

- Events: `PreToolUse`, `PermissionRequest`, `PostToolUse`, `UserPromptSubmit`, `Stop`, `SubagentStop`, `PreCompact`, `PostCompact`, `Interrupt`, `SessionStart`, `SessionEnd`, `SubagentStart`.
- Config: `hooks.json` or inline `[hooks]` in `config.toml`, at `~/.codex/` and `<repo>/.codex/`. **All matching hooks from all layers run** (no override). Project hooks load only when the project `.codex/` layer is **trusted**.
- **Trust review**: non-managed hooks must be reviewed and trusted by the user against the **hook definition hash**. New or changed hooks are skipped until trusted (`/hooks`). Managed hooks (system, MDM, `requirements.toml`) are trusted by policy.
- Handler types: `command` and `mcp_tool` work. `prompt` and `agent` handlers are parsed but **skipped**. `commandWindows` override exists.
- Tool coverage for `PreToolUse`/`PostToolUse`: shell (as `Bash`), unified exec, `apply_patch` (matchable as `apply_patch`/`Edit`/`Write`; `tool_input.command` holds the patch), MCP tools, other local function tools. **Hosted tools such as `WebSearch` are not covered.** The docs state: "Treat tool hooks as a useful guardrail, not a complete enforcement boundary." `write_stdin` does not re-trigger `PreToolUse`.
- `PreToolUse` output: `deny` (or legacy `decision: "block"`, or exit 2) blocks. `allow` + `updatedInput` rewrites. **`ask`, `continue:false`, `stopReason`, `suppressOutput` are unsupported: the hook is marked failed and the tool call continues.** Plain stdout is ignored.
- `PermissionRequest` fires only when Codex is about to ask for approval. Hook can `allow`, `deny`, or abstain (normal prompt appears). Any deny wins.
- `Stop`: needs JSON on exit 0. `decision: "block"` does not reject; it **creates a continuation prompt using `reason` as new user text**. `stop_hook_active` is provided. I did not find a documented continuation cap (**[UNVERIFIED]**; assume none and implement our own).
- Input fields: `session_id`, `transcript_path` ("format isn't a stable interface"), `cwd`, `model`, `permission_mode`, `turn_id`. Docs say a `PreToolUse` callback error, timeout, or malformed response "can fail the hook without blocking the tool" (for managed MCP hooks; assume the same for command hooks).
- Default timeout 600 s for most hooks.
- **[STILL UNVERIFIED]**: Codex hook behaviour on native Windows. The Windows page (https://developers.openai.com/codex/windows.md) documents a native sandbox and the hooks page documents `commandWindows`/`windows_managed_dir`, but nothing states that hooks fire under the native Windows sandbox, which user the hook command runs as, or whether it runs inside the sandbox. This is a Phase 0 experiment.
- **(v0.2) Codex config facts** (https://developers.openai.com/codex/config-reference.md, CLI reference): `approval_policy` is `on-request`, `never`, or `granular` (`untrusted` is no longer supported, `on-failure` deprecated); `sandbox_mode` and named permission profiles (`:read-only`, `:workspace`, `:danger-full-access`); `model_reasoning_effort` and `model` are config keys; any key can be overridden per invocation with `-c key=value`, so a launcher can set model/effort via `-c`. `features.hooks` enables hooks (`codex_hooks` deprecated alias); admins can pin it in `requirements.toml`. `codex execpolicy` and command `rules` exist (native allow/prompt/forbid rules for commands). I did not confirm a `-m/--model` CLI flag in the rendered docs (the flag tables did not render); use `-c model=...` until verified.

### 1.3 Capability matrix and what it implies

| Capability | Claude Code | Codex | Wardent implication |
| :- | :- | :- | :- |
| Block a tool call | Yes (exit 2 / `deny`) | Yes (exit 2 / `deny`) | Same core decision, per-adapter encoding |
| "Ask the user" from PreToolUse | Yes (`ask`) | **No** | Codex: map `ask` to `PermissionRequest` abstain (only works if Codex would prompt anyway) or to `deny` with an actionable reason, per policy |
| Rewrite tool input | Yes | Yes (Bash/apply_patch need string `command`) | Not used in v1; possible later (e.g. add `--dry-run`) |
| Block prompt | Yes | Yes | Used only for secrets in prompt |
| Inject context | `additionalContext` | `additionalContext` / plain stdout | Used for Scope Guard reminders |
| Force continuation at Stop | Yes, capped at 8 | Yes (continuation prompt) | Verifier; we track our own loop counter and escalate to the user |
| Change model/effort | **No** | **No** (none found) | Router is advisory/launcher only |
| Failure semantics | Fail-open on timeout/crash/bad output | Fail-open on error | Shim must catch its own failures and emit a deny/ask itself for high-risk categories; see section 5 |
| Hook tamper detection | `ConfigChange`, managed settings | Trust hash, managed hooks | `wardent doctor` verifies hooks present and unchanged |
| Hook-free file coverage | `@` refs bypass `PreToolUse` | hosted tools bypass | Document gaps, recommend native deny rules as complement |

### 1.4 Recommended integration approach

**Primary: command hooks that call `wardent hook <agent> <event>`.**
- Works on both agents (Codex has no `http` handler).
- The hook command string must be **stable across Wardent upgrades** (e.g. `wardent hook claude pre-tool-use`), so Codex's trust hash is not invalidated on every update. Behaviour changes live inside the binary, not in the config string.
- `wardent install` writes user-level config by default (`~/.claude/settings.json`, `~/.codex/hooks.json`) and offers a project-level option (committed, for teams). It tells Codex users they must approve the hook in `/hooks`.

**Optional later: `http` hook to a local daemon for Claude Code** to avoid process spawn. It adds a new fail-open mode (connection refused is non-blocking) and `allowedHttpHookUrls` interplay, so it is not v1.

**Complementary, not replaced:** Wardent should also generate recommended native config (Claude `permissions.deny` for `Read(.env*)`, since `@` refs bypass hooks; Codex sandbox/approval settings). Hooks plus native rules gives defence in depth.

**Rejected alternatives**
- *PTY/stdio wrapper around the agent*: fragile, breaks TUIs, cannot see structured tool calls.
- *MITM proxy on the model API*: violates "local-first, no TLS tricks", breaks with agent updates, and sees no tool execution results.
- *Transcript tailing only*: after-the-fact, no blocking. Useful as a Phase-1 observer fallback, and for the Loop Detector as a second signal. Both vendors say transcript format is not a stable interface.
- *Agent SDK embedding*: Claude Agent SDK callback hooks fail closed on timeout (a nice property), but this only covers users who build on the SDK, not interactive Claude Code or Codex CLI.

### 1.5 Sources fetched
- Claude Code hooks reference: https://code.claude.com/docs/en/hooks (events table, exit-code semantics, timeouts, PreToolUse and Stop decision control, PreModelSwitch)
- Claude Code permissions: https://code.claude.com/docs/en/permissions (deny/ask/allow ordering, "enforced by Claude Code, not by the model")
- Codex hooks: https://developers.openai.com/codex/hooks (events, trust review, tool coverage, PreToolUse/PermissionRequest/Stop contracts, `ask` unsupported)
- Read in v0.2: Claude CLI reference (https://code.claude.com/docs/en/cli-reference), model config (https://code.claude.com/docs/en/model-config), permission modes (https://code.claude.com/docs/en/permission-modes), sandboxing (https://code.claude.com/docs/en/sandboxing), Codex config reference, Codex CLI reference, Codex Windows (https://developers.openai.com/codex/windows).
- **Still not read** (assigned to Phase 0 as verification tasks, because they affect install design): Claude `settings` and `settings-reference` pages in full (settings precedence, `hooks` key schema, managed-settings paths per OS), `auto-mode-config`, Codex `agent-approvals-security` and `rules` pages, Codex CLI global flag tables (did not render as text).
- Caveat on all of the above: pages were fetched through a text conversion and large parts were truncated in my viewer; claims cite specific passages I read, but the spike re-verifies behaviour empirically rather than trusting docs alone.

---

### 1.6 First adapter, given the access the project actually has (added 2026-10-04)

> **SUPERSEDED (v0.3, 2026-10-07): the first and only release-1 adapter is Devin CLI.** The text below is kept as the reasoning for why unverifiable adapters must not ship enforcement; it now applies to Claude Code and Codex as future adapters.

Facts: the developer has **no Claude Code account**, develops on **native Windows**, and Codex access is **unconfirmed** (`codex` is not installed on the dev machine; the question was left unanswered). The earlier plan assumed Claude Code first, Codex second.

**Decision rule (an adapter may not ship enforcement unless it has been verified end to end on a real agent):**

| Situation | First adapter | Second |
| :- | :- | :- |
| Codex access confirmed (ChatGPT/Codex login usable here) | **Codex** | Claude Code, after someone with an account runs E1/E3 |
| Claude Code access obtained | **Claude Code** (original plan) | Codex |
| Neither available | **None can be verified.** Phase 1 would be limited to a docs-derived adapter shipped as clearly labeled *experimental, observe-only*; no `deny`/`ask` enforcement until verified. | |

**Recommendation:** do not ship a Claude adapter that was written only from documentation and has never run against the real binary. The Claude hook contract has many fail-open traps (section 1.1) whose actual behaviour E3 exists to confirm. Obtaining access to **one** agent is the highest-value unblocker for Phase 1.

**What changes if Codex is first (it may be Windows-native friendly, but that is still unverified, E7):**
1. `PreToolUse` cannot `ask`, so "fail-safe prompting" degrades per `[codex].ask_fallback`; release 1's ask semantics are weaker than the Claude plan (`deny` for critical, `abstain`+`PermissionRequest` context otherwise). Observe-first still works unchanged.
2. No HTTP hook: a **command shim is mandatory**, which strengthens the Go decision (spawn cost on every call; see `reports/E5-coldstart.md`).
3. Install must handle Codex's **trust review** (hooks are skipped until the user approves them in `/hooks`; changed definitions need re-approval). E4 must confirm what the trust hash covers before the command-string-stability assumption is relied upon.
4. File edits arrive as `apply_patch` text; the adapter needs a patch parser (paths and ops) instead of Claude's structured `file_path`.
5. Hosted tools (web search) are not hookable; documented gap.
6. Native Windows hook behaviour under Codex's sandbox is unknown (E7) and now sits on the critical path.
7. The Claude-specific findings in section 1.1 (project-level `disableAllHooks`, auto-mode overlap, no native Windows sandbox) remain relevant to the second adapter and to positioning (section 11), but cannot be validated until someone has Claude Code.
8. Differentiation (section 11) shifts: against Codex's native sandbox/approvals and rules, the "native Windows has no Claude sandbox" argument no longer applies to the first adapter.

---

## 2. Architecture and module boundaries

### 2.1 Process model (latency-driven)

```
agent (Claude Code / Codex)
   │ spawns per event, JSON on stdin
   ▼
wardent hook <agent> <event>        <- "shim": same binary as CLI, tiny startup
   │ 1. adapter.parse -> AgentEvent
   │ 2. load policy (mtime-cached) + session state
   │ 3. TIER 0: deterministic rules in-process          (target p95 < 15 ms)
   │ 4. if uncertain AND budget allows -> ask daemon     (TIER 1, model)
   │ 5. policy engine combines -> Decision
   │ 6. audit.append (sync, fsync batched)
   │ 7. adapter.encode(Decision) -> stdout/exit code
   ▼
wardentd (optional, lazily started, per-user)
   - holds Laya model warm, session state cache
   - unix socket / Windows named pipe, user-only ACL
   - never reachable over the network
```

- **The rules path never depends on the daemon.** If the daemon is down or slow, the shim proceeds with tier 0 only (section 5).
- Model calls are made only where semantic judgment is worth the latency, with per-event budgets:

| Event | Tier-1 model allowed? | Budget (initial guess, to be measured) |
| :- | :- | :- |
| `PreToolUse` Bash/Edit | Only when tier 0 says "uncertain" | 150 to 300 ms hard cap |
| `UserPromptSubmit` | Yes (Router, intent extraction) | 500 ms |
| `Stop` | Yes (claim vs evidence) | 1 to 2 s |
| `PostToolUse` | No (record only, async enrich) | n/a |

### 2.2 Modules

| Module | Responsibility | Depends on | Must NOT |
| :- | :- | :- | :- |
| `core` | Engine pipeline, orchestrates detectors, owns schemas | `policy`, `provider` (interface), `audit` | Know any agent-specific field names |
| `detectors` | Four capabilities as pure functions `(AgentEvent, Session, Policy) -> Evidence[]`: `risk`, `scope`, `loop`, `verify`, `route` | `core` schemas, `shellparse` | Make final decisions; do I/O |
| `shellparse` | Bash/zsh, PowerShell, cmd tokenizer and command normalizer (resolve `sudo`, `env`, pipes, subshells, `-c` wrappers, path normalization) | none | Execute anything |
| `policy` | Loads/validates policy files, merges layers, evaluates rules, combines Evidence into a Decision (section 4) | `core` schemas | Call the model directly |
| `provider` | `DecisionProvider` interface plus implementations: `NullProvider`, `LayaProvider` (local), later `OllamaProvider`/remote opt-in | schemas | Be on the critical path without a deadline |
| `adapters/<agent>` | Parse native hook JSON to `AgentEvent`; encode `Decision` to native output; install/uninstall/doctor | `core` schemas | Contain policy logic |
| `session` | Per-session state store: stated intent, touched files, evidence ledger, loop counters, continuation counts | `core` | Store raw file contents |
| `audit` | Append-only local log, redaction, rotation, query | `core` | Ever write secrets in clear |
| `config` | Resolve user/repo/managed config layers, paths per OS | none | |
| `cli` | `install`, `uninstall`, `doctor`, `hook`, `log`, `explain`, `policy check`, `eval`, `run` (launcher) | all | |
| `daemon` | Warm model host and cache | `provider`, `session` | Be required |
| `eval` | Labeled dataset runner and metrics (section 6) | `core`, `policy`, `provider` | Ship in the runtime binary's hot path |

**Dependency rule:** `adapters -> core <- policy`, `detectors -> core`, nothing in `core` imports an adapter. This is what makes "Codex second" cheap.

### 2.3 Provider interface (behavioural contract)

```
DecisionProvider
  capabilities(): { tasks: [intent_extract | scope_judge | risk_judge | claim_verify | route], max_input_tokens, typical_latency_ms }
  judge(task, input, deadline_ms): Judgment | ProviderError
  health(): ok | degraded | down

Judgment { task, label, confidence 0..1, rationale (short, optional), provider_id, model_version, latency_ms }
```

Contract rules:
- Always called with a **deadline**; must return or error by then.
- Input is a **minimized, redacted** context (policy-controlled), never the whole repo.
- Output is **structured and enumerated** (label from a closed set), never free-form commands.
- Providers are stateless from Wardent's view. Multiple providers can be chained or voted. A provider may declare `tasks` it does not support.
- `NullProvider` is first-class: Wardent must work fully (with lower semantic recall) with it. This is also the baseline in evals.

### 2.4 Decision encoding per adapter (the "ask" problem)

| Core decision | Claude Code encoding | Codex encoding |
| :- | :- | :- |
| `allow-silent` (abstain) | exit 0, no output | exit 0, no output |
| `warn` | exit 0 + `systemMessage` and/or `additionalContext` | same (`systemMessage` supported on PreToolUse) |
| `ask` | `permissionDecision: "ask"` + reason | Not supported in `PreToolUse`. Policy-selectable: `ask_fallback = "deny" \| "abstain"`. Default `deny` with message "Wardent needs confirmation: run `wardent allow <id>` or approve in repo policy". Also register a `PermissionRequest` hook so cases where Codex would prompt anyway are surfaced with context |
| `deny` | `permissionDecision: "deny"` + reason (shown to Claude) | `permissionDecision: "deny"` or exit 2 |
| `block-stop` (Verifier) | `decision: "block"` + `reason` | `decision: "block"` + `reason` (becomes new prompt text) |

---

## 3. Core schemas

Notation: TypeScript-style type sketches for documentation only. The on-disk/wire format is JSON with an explicit `schema_version`. Chosen implementation language may differ (section 7).

### 3.1 AgentEvent (agent-agnostic, produced by adapters)

```
AgentEvent {
  schema_version: 1
  event_id: string               // ULID
  ts: ISO8601
  agent: { kind: "claude-code" | "codex" | string, version?: string, adapter_version: string }
  session_id: string             // agent's session id, namespaced by agent kind
  turn_id?: string
  subagent?: { id: string, type?: string }
  cwd: string                    // normalized absolute path
  repo_root?: string
  permission_mode?: "default"|"plan"|"acceptEdits"|"auto"|"dontAsk"|"bypassPermissions"
  model?: string
  effort?: string
  kind: "prompt_submitted" | "tool_pre" | "tool_post" | "tool_failed" | "turn_stop" | "session_start" | "session_end" | "permission_request" | "config_change"
  prompt?: { text: string }
  tool?: {
    id: string                   // tool_use_id
    name: string                 // raw agent tool name
    class: "shell" | "file_write" | "file_read" | "file_patch" | "network" | "mcp" | "agent" | "other"   // normalized
    shell?: { dialect: "posix"|"powershell"|"cmd", command: string }
    files?: [{ path: string /*absolute, normalized, forward-slash internal*/, op: "read"|"create"|"modify"|"delete" }]
    patch_text?: string          // for patch-style tools
    mcp?: { server: string, tool: string, source?: string }
    raw_input_digest: string     // hash only; raw kept in memory, not persisted by default
  }
  result?: { ok: boolean, exit_code?: number, output_excerpt?: string /*bounded, redacted*/, changed_files?: string[] }
  stop?: { assistant_message?: string, stop_hook_active: boolean }
  raw_ref?: string               // pointer to opt-in raw payload capture (debug only)
}
```

Normalization is the adapters' job: `Edit/Write/MultiEdit` (Claude) and `apply_patch` (Codex) both become `file_write`/`file_patch` with `files[]`. Windows paths are normalized once, here.

### 3.2 Evidence (one detector's finding; many per event)

```
Evidence {
  id: string
  event_id: string
  source: { type: "rule"|"model"|"heuristic"|"state", id: string, version: string }   // rule id, provider id, detector id
  capability: "route"|"scope"|"risk"|"loop"|"verify"
  category: string               // e.g. "destructive_fs", "secret_exposure", "protected_path", "out_of_scope_edit", "repeat_failure", "unverified_claim"
  severity: "info"|"low"|"medium"|"high"|"critical"
  confidence: number | null      // null for deterministic rules (certain by construction)
  determinism: "hard" | "soft"   // hard: a policy/rule that is authoritative; soft: advisory, may be overridden by model/user
  subject: { kind: "command"|"path"|"prompt"|"claim"|"sequence", ref: string, span?: [number,number] }
  detail: string                 // human-readable, already redacted
  data?: object                  // structured extras (matched pattern, similarity score, etc.)
}
```

### 3.3 Decision

```
Decision {
  schema_version: 1
  decision_id: string
  event_id: string
  action: "allow_silent" | "warn" | "ask" | "deny" | "block_stop" | "inject_context"
  reason: string                           // one or two sentences, shown to user (and to the agent on deny)
  explanation: {                           // for `wardent explain`
    evidence_ids: string[]
    deciding_rule?: string                 // policy rule id or built-in rule id
    combination: "rule_hard" | "rule_soft+model" | "model_escalation" | "fallback_rules_only" | "fail_safe" | "default"
    model_used: { provider: string, version: string, label: string, confidence: number } | null
    overridden: [{ evidence_id: string, by: string }]   // what was overridden and why
  }
  degraded?: { reason: "model_unavailable"|"model_timeout"|"low_confidence"|"policy_error"|"internal_error"|"daemon_down" }
  user_prompt?: { choices: string[] }      // for ask
  latency_ms: { total: number, tier0: number, model?: number }
  policy: { source_files: string[], hash: string }
}
```

### 3.4 Policy: see section 4.

### 3.5 Session (state across events)

```
Session {
  key: "<agent>:<session_id>"
  started_at, last_event_at
  repo_root, agent, model_history: [...]
  intent: {                                  // from prompt_submitted
    latest_prompt_digest, summary?: string, // summary only if model available; else keywords
    allowed_scope: { path_globs: string[], inferred_by: "prompt_paths"|"model"|"user_policy" }
    turn_index: number
  }
  touched: { files: Map<path, {ops, first_ts, last_ts, count}> }
  ledger: [{ tool_class, normalized_command?, ts, ok, exit_code, paths }]   // bounded ring buffer
  verification: { last_test_cmd?: {cmd, ok, ts, after_last_write: boolean}, last_build?: ..., last_write_ts }
  loop: { recent_signatures: [...], repeat_counts: Map<signature, n>, no_progress_turns: n }
  continuations: { stop_blocks_this_turn: n, total: n }
  user_grants: [{ pattern, expires_at, scope: "session"|"repo" }]
}
```

Persisted in a small local store (SQLite or an append-only file plus snapshot); keyed by session; pruned by age. Stores digests and normalized commands, not file contents.

### 3.6 Audit record

One line per decision (JSONL) containing the `Decision`, the referenced `Evidence[]`, the redacted `AgentEvent` summary, `policy.hash`, `wardent_version`, `adapter_version`, `agent.version`. Secrets are redacted before writing (see risk 6).

---

## 4. Policy file format and combination logic

### 4.1 Layers and location

Merge order, lowest to highest precedence:
1. Built-in defaults (shipped, versioned)
2. User: `~/.config/wardent/policy.toml` (`%APPDATA%\wardent\` on Windows)
3. Repo: `<repo>/.wardent/policy.toml` (committed) and `<repo>/.wardent/policy.local.toml` (gitignored)
4. Managed (later): system path, admin-controlled, can mark rules `locked`

**Tightening is always allowed from any layer; loosening below a `locked` rule is not.** A repo cannot silently disable a built-in critical rule unless a higher (user) layer permits it (this is a supply-chain defence: a cloned repo must not be able to weaken protections).

TOML is proposed (comments, human-edited, matches Codex config), with a published JSON Schema and `wardent policy check`. YAML is an acceptable alternative; the choice is not load-bearing.

### 4.2 Sketch

```toml
version = 1

[mode]
default = "observe"            # observe | warn | enforce   (v0.2: observe-first onboarding for individuals)
failure_stance = "prompt"      # prompt (default) | strict   (strict = fail closed on high-risk classes, opt-in)
unknown_agent_version = "warn" # adapter drift behaviour

[thresholds]
model_min_confidence = 0.80    # below this the model's label is ignored
scope_deviation_ask  = 0.70
loop_repeat_count    = 3
loop_no_progress_turns = 4
latency_budget_ms    = { pre_tool = 250, prompt = 500, stop = 1500 }

[protected_paths]
deny_write = [".git/**", ".github/workflows/**", "**/.env*", "infra/prod/**"]
ask_write  = ["package.json", "**/migrations/**"]
deny_read  = ["**/.env*", "**/*.pem", "~/.ssh/**", "~/.aws/**"]

[[rules]]
id      = "no-force-push"
match   = { tool = "shell", command_regex = '^git\s+push\b.*(--force|-f)\b' }
action  = "deny"
reason  = "Force-push is disabled in this repo."
locked  = false

[[rules]]
id      = "allow-test-commands"
match   = { tool = "shell", argv_prefix = ["npm", "test"] }
action  = "abstain"            # explicit no-opinion; also exempts from soft model escalation
trust   = "high"

[risk]
secrets = { scan_prompts = true, scan_tool_inputs = true, scan_outputs = true, patterns = "builtin+custom" }
network = { allow_hosts = ["registry.npmjs.org", "github.com"], unknown_host = "ask" }

[scope]
enabled = true
mode    = "ask"                # warn | ask
always_in_scope = ["tests/**", "docs/**"]
never_in_scope  = ["LICENSE"]

[verify]
require_test_after_edit = true
claim_triggers = ["tests pass", "fixed", "done", "all green"]
test_commands = ["npm test", "pnpm test", "pytest", "go test ./..."]
max_continuations = 2          # below Claude's cap of 8, then escalate to the user

[model]
enabled = true
provider = "laya-local"
share_context = "minimal"      # none | minimal | snippets   (never "full" by default)
allow_model_to = ["escalate"]  # escalate | de-escalate_soft

[codex]
ask_fallback = "deny"          # deny | abstain
```

### 4.3 Combination algorithm (deterministic, explainable)

Inputs: `Evidence[]` from rule detectors (tier 0), optionally `Judgment` from the provider (tier 1), the policy, the session.

1. **Evidence collection.** Each detector yields `Evidence` with `determinism: hard|soft`. Policy `deny` rules, protected paths, and critical secret matches are `hard`.
2. **Hard rules first.** If any `hard` evidence maps to `deny`, the decision is `deny`. **The model is never consulted to overturn it** and cannot de-escalate it. If a `hard` rule maps to `ask`, the model cannot downgrade it either.
3. **Model is consulted only if**: (a) there is `soft` evidence or no evidence but the event class is flagged semantic (Scope, Verify, Route), (b) budget remains, (c) provider is healthy and supports the task.
4. **Model output handling**:
   - `confidence < model_min_confidence`: ignored, and recorded as `low_confidence`. Falls back to the rules-only result.
   - Model **escalation** (rules say allow/warn, model says risky with confidence ≥ threshold): allowed up to `ask` by default. Up to `deny` only if policy opts in (`allow_model_to` plus category allowed to be model-denied).
   - Model **de-escalation** of `soft` evidence: only if `allow_model_to` contains `de-escalate_soft`, only for categories not marked `critical`.
5. **Severity to action table** (policy-overridable):

| Max severity of effective evidence | `observe` | `warn` | `enforce` |
| :- | :- | :- | :- |
| none | allow_silent | allow_silent | allow_silent |
| low | allow_silent | warn | warn |
| medium | warn | warn | ask |
| high | warn | ask | ask (deny if hard rule says so) |
| critical | warn | ask | deny |

6. **User grants** (`wardent allow ...` or answering `ask` in Claude) can downgrade `ask` and `soft` outcomes for a scope/time, but never `hard`+`critical` ones unless the policy permits user override.
7. **The result must always carry**: deciding rule or provider, evidence ids, and a combination label. If a decision cannot be explained from `explanation`, that is a bug (test asserts it).

---

## 5. Failure modes

Principle: **no failure may produce a silent, undetected pass on high-risk actions, and no failure may block the developer indefinitely.** Always record the degraded state in the audit log and tell the user once per session, not per call.

**Failure stance (decided in v0.2):** default `failure_stance = "prompt"`: on uncertainty or internal failure, `ask`/`warn` with a visible degraded notice; **deny only for critical hard rules**. Opt-in `failure_stance = "strict"`: on internal failure or uncertainty for high-risk classes (shell commands, writes to protected paths), fail closed. On Codex, "ask" is not available in `PreToolUse`, so default-mode asks degrade per `[codex].ask_fallback`, and strict mode denies. In both stances, a crash of the shim must never rely on the vendor default (which is fail-open).

| Failure | Detection | Behaviour | Why |
| :- | :- | :- | :- |
| **Laya down / not installed / crashed** | `provider.health()`, connect error | Rules-only decision, `degraded=model_unavailable`. Semantic checks (Scope, Verify claim check) fall back to heuristics (path allowlist from prompt, "test ran after last write?") and **downgrade to `warn`**, not `deny`. One-time notice. | Rules are the core. Model is an input. |
| **Model slow** | deadline exceeded | Use tier-0 result. Never wait past budget. | Latency constraint. |
| **Low confidence** | `confidence < threshold` | Ignore the label. For Scope and Verify: `ask`/`warn` per policy. Never auto-allow something rules flagged `soft` high severity. | Fail safe = ask, not guess. |
| **Model contradicts hard rule** | combination step | Hard rule wins, contradiction logged for eval mining. | Model never final authority. |
| **Wardent shim crashes / panics** | top-level handler in shim | Catch everything. For `PreToolUse`, emit **explicit `ask` (Claude) or `deny` (Codex, per `ask_fallback`) only if the event is in a "dangerous class" by a trivial pre-check** (shell tool, writes to protected paths); otherwise exit 0 and log. | Vendor semantics make crashes fail-open (exit 1 does not block); the shim must convert internal errors into an explicit decision itself. |
| **Shim not found / path wrong / not executable** | `wardent doctor`; `SessionStart` self-check hook; `ConfigChange` hook (Claude) | Vendor fails open here (shell exit 127 is non-blocking). Mitigation: install writes an absolute, verified path (or a stable launcher on PATH), `doctor` runs a live round-trip, and `SessionStart` hook warns if broken. | Documented silent-disable trap. |
| **Hook timeout (vendor side)** | Our own internal deadline ≪ vendor timeout (e.g. 2 s) | Always answer before the vendor does. Set explicit `timeout` in installed config. | Vendor timeouts do not block `PreToolUse`. |
| **Policy file invalid** | `policy check` at load | Use last-known-good cached policy. If none: built-in defaults in `warn` mode plus visible notice. Invalid repo policy never loosens anything. | Safe + non-bricking. |
| **Adapter breaks after agent update** | (a) schema validation of incoming JSON against pinned fixtures, unknown required fields missing, (b) `agent.version` outside tested range, (c) canary CI | Tolerant parsing (ignore unknown fields). On parse failure: classify as `adapter_error`, fall back to a **generic extractor** that regex-searches known fields (`tool_input.command`, `file_path`); if still unusable, emit `warn` message "Wardent can't read this event, protections reduced", record in audit, and surface in `doctor`. Never crash the agent. | Agents change monthly (hooks docs already reference versions v2.1.2xx with new fields). |
| **Agent changes hook semantics** (e.g. new decision fields) | Contract tests against recorded real outputs, canary job installing latest agent | Version-gated encoders: adapter selects encoder by detected agent version; unknown newer version uses last known encoder and logs. | |
| **Codex trust hash invalidated / hook not trusted** | Hook never invoked, so `doctor` and Codex `/hooks` | Install prints instructions. Keep command string stable. `doctor` checks that events actually arrive (heartbeat in audit log). | Codex skips untrusted hooks silently-ish. |
| **Stop-hook loop** (verifier keeps blocking) | `continuations` counter | Stop blocking after `max_continuations` (lower than Claude's 8) and show the user a final "unverified" notice instead. | Prevents token burn and the vendor override. |
| **Audit log unwritable / disk full** | write error | Do not block the agent. Keep in-memory ring buffer, emit one warning. For `enforce`+critical deny decisions still enforce. | Auditability matters, but not at the cost of bricking. |
| **Concurrent sessions / subagents** | `agent_id`, session key | Per-session state with file locking. Subagent events attributed to parent session plus `subagent`. | |
| **Hook bypass (`@file`, hosted tools, obfuscated shell)** | n/a (undetectable by hook) | Documented limitations, recommended native deny rules generated by `wardent install --recommend`. | Honesty about the boundary. |

---

## 6. Testing strategy

### 6.1 Layers

1. **Unit tests**: rules, policy merge/precedence, severity table, path normalization (Windows, macOS case-insensitivity, symlinks, `..`), secret patterns.
2. **Property/fuzz tests**: `shellparse` (random quoting, unicode, here-docs, `$(...)`, backticks, PowerShell encodings such as `-EncodedCommand`) must never panic and must produce a conservative result on unparseable input ("unparseable shell with dangerous token ⇒ medium risk").
3. **Adapter contract tests**: recorded real hook payloads and real agent output expectations per agent version, stored as fixtures (`fixtures/claude-code/2.1.x/pre-tool-use-bash.json`). Tests check parse to AgentEvent and Decision to encoded output, including exit code semantics (e.g. "deny never encoded as exit 1").
4. **Agent-in-the-loop integration tests**: scripted headless runs (`claude -p`, `codex exec`, **[UNVERIFIED flags]**) against a fixture repo using a deterministic fake provider, asserting that a dangerous command is actually blocked end to end. Run on all 3 OSes.
5. **Canary job (nightly)**: install the latest published Claude Code and Codex, replay the contract suite and an e2e smoke, open an issue on failure. This is the main defence against "adapter breaks after agent update".
6. **Latency benchmarks in CI**: hook cold-start p50/p95 per OS, tier-0 evaluation time, regression budgets (e.g. fail the build if p95 cold path > budget).
7. **Chaos tests**: kill daemon mid-request, corrupt policy, slow provider, full disk, malformed stdin, huge payloads (1 MB prompt), and assert on fail-safe behaviour from section 5.
8. **Policy golden tests**: `policy + event -> expected Decision.explanation`. Explanations are part of the contract.

### 6.2 Labeled evaluation set (decision quality)

**Unit of evaluation:** `(AgentEvent sequence with session context, policy) -> expected {action, category, severity}`, labeled by humans. This evaluates the *whole pipeline* (rules + model + combination), plus rules-only and model-only ablations.

**Format:** JSONL, versioned, one case per line:
```
{ id, capability, scenario_tag, events: [...], session_context: {...}, policy_ref,
  label: { action, category, severity, acceptable_actions: [...] },
  rationale, source: "synthetic|public|dogfood|adversarial", labeler_ids: [...], agreement: 0..1, split: "dev|test|holdout" }
```

**Strata and targets** (initial; sizes to grow):

| Capability | Positive classes | Hard negatives (must NOT flag) | Initial size |
| :- | :- | :- | :- |
| Risk | `rm -rf` variants, `git push --force`, `curl|sh`, `chmod -R 777`, writing `.env`, printing secrets, `DROP TABLE`, cloud CLI destructive ops, obfuscated (base64, `$IFS`, env var indirection, PowerShell aliases) | `rm -rf node_modules`, `rm -rf ./build`, `git push` normal, tests that mention "password" fixtures, `.env.example` | 600 |
| Scope | Edit unrelated module, drive-by refactor, touching lockfiles/CI/configs, deleting tests | Legit collateral edits (tests for the change, imports, generated files, renames) | 400 |
| Loop | Same failing command ×N, edit-revert oscillation, no-progress retry | Legit repetition (polling a build, TDD red/green cycles, retry after fix) | 200 sequences |
| Verify | "done/tests pass" claims with no test run, test run before last edit, failing exit code ignored | Claims backed by passing test after last write, claims about non-testable tasks (docs) | 300 |
| Route | Task descriptions labeled with the "right" tier/effort by experienced devs | n/a, evaluated as agreement and cost/quality tradeoff | 200 |

**Sourcing**: (a) synthetic generation seeded by hand-written templates, reviewed by humans; (b) public red-team command lists and published incident reports; (c) **opt-in, redacted dogfood traces** from our own use (the highest value for false-positive measurement); (d) adversarial set written by someone who has seen the rules (bypass attempts), refreshed regularly. No customer data without explicit, per-trace consent and redaction.

**Labeling**: two independent labelers per case, a third resolves disagreements. Track inter-annotator agreement per stratum. Cases with low agreement stay in a "contested" bucket (reported, not used for gating). Keep a frozen **holdout** never used for tuning rules or prompts, and rotate fresh cases in each release to detect overfitting.

**Metrics (reported per capability, per OS shell dialect, per provider config)**:
- **Critical-risk recall** (misses are the worst error). Gate: ≥ 0.99 on `critical`, with rules-only already achieving the bulk.
- **False-interrupt rate** = `ask`/`deny` per 100 benign tool calls on dogfood traces. Gate to be set from user research (hypothesis: < 1 per 100, since fatigue leads users to uninstall).
- Precision/recall per category, calibration curve for model confidence (is 0.8 really 80%?), and **abstention quality** (accuracy when the model is above threshold vs. coverage).
- **Safety-asymmetric regressions**: any case with a `critical` label that flips to `allow` fails the build regardless of aggregate score.
- Latency distribution per tier and per provider.
- Ablations: rules-only, model-only, combined. The model must demonstrate **lift over rules-only** on Scope/Verify or we do not ship it for that capability.

**CI gating**: `wardent eval` runs the dev split on every PR (fast, rules-only plus recorded model outputs), full run with live provider nightly. Model/prompt/provider changes require eval report diffs in the PR.

---

## 7. Repo layout, language, tooling

### 7.1 Language: challenging "TypeScript assumed"

**What the product needs from the language:**
1. Cold start of the hook process per tool call (Claude can fire many per turn; parallel batches too).
2. Single-file install on macOS/Linux/Windows without a prerequisite runtime.
3. Safe, fast parsing of untrusted input (shell strings).
4. A local model runtime story (llama.cpp / ONNX Runtime / Ollama).
5. Team velocity and ecosystem for schemas, CLI, tests.

| Option | Cold start (hook path) | Distribution | Model runtime | Notes |
| :- | :- | :- | :- | :- |
| **TypeScript on Node** | Typically tens to 100+ ms per spawn before any work (**to be measured, not asserted**) | npm: requires Node, one-command install is easy if user has Node | Via native bindings or HTTP to Ollama/llama.cpp | Best schema/JSON ergonomics (zod), easiest hiring, reuse for future extension/web UI. Both target agents are Node/TS-ish ecosystems, so Node is present for most users today (not guaranteed for Codex users since it is distributed in other forms too) |
| TypeScript compiled (Bun/Deno single binary) | Better than Node but larger binary (~50-100 MB) and still not minimal | Single binary | Same | Reasonable middle path |
| **Go** | ~ms | Static binary per OS/arch, trivial cross-compile, good Windows story | cgo to llama.cpp is painful; simpler to use a sidecar process/HTTP | **My recommendation** for CLI/shim/rules/daemon |
| Rust | ~ms | Static binary | Best native bindings (llama.cpp, ONNX) | Slower velocity, steeper hiring. Strong if Laya runs in-process |
| Python | Slow start | Poor | Best for training/eval | Use for eval/data tooling only, not runtime |

**Recommendation:** Go for the runtime (`wardent` binary: CLI, hook shim, rules, daemon) because the hook path is latency-critical and spawn-per-call, and a no-prerequisite installer is a product constraint. Keep the Laya runtime as a **sidecar process** speaking a small local protocol, which also fits the "swap or supplement" provider requirement and keeps cgo out of the core. Use Python (or TS) for dataset tooling and training pipelines, communicating via the JSONL formats in section 6.

**Decision procedure instead of taste:** Phase 0 spike measures cold-start and tier-0 evaluation time for Node, Bun-compiled and Go on all three OSes. If Node's p95 spawn-to-decision is within budget (say < 80 ms) *and* the team is TS-only, TypeScript is acceptable provided the daemon is mandatory-by-default and the shim is a minimal launcher. If not, Go. I am not asserting the Node number; it must be measured.

### 7.2 Tooling (assuming Go; mirrored for TS)

- Schemas defined once as **JSON Schema** (source of truth), generated into Go types (and TS/Python types for tooling). Wire formats and audit log are versioned.
- Task runner: `just` or `make`. Lint/format: `golangci-lint`, `gofumpt`. Tests: `go test`, fuzzing via Go native fuzz.
- Release: GoReleaser to GitHub Releases, Homebrew tap, Scoop/winget, `curl | sh` and PowerShell installers, optional `npm i -g wardent` wrapper that downloads the binary (for TS-centric users). Signed binaries, SBOM, checksum verification in the installer. macOS notarization and Windows code signing are required for a commercial product (budget this).
- CI matrix: ubuntu, macOS (arm64 + x64), windows. Nightly canary jobs (section 6).

### 7.3 Repo layout

```
wardent/
  docs/                      design, ADRs, policy reference, adapter notes (per agent version)
  schemas/                   JSON Schemas: agent_event, evidence, decision, policy, session, audit
  cmd/wardent/               main (CLI + hook shim + daemon subcommand)
  internal/
    core/                    pipeline, orchestration
    detectors/{risk,scope,loop,verify,route}/
    shellparse/{posix,powershell,cmd}/
    policy/                  load, merge, evaluate, combine
    provider/                interface, null, laya (sidecar client), mock
    adapters/{claudecode,codex}/   parse, encode, install, doctor, fixtures
    session/                 state store
    audit/                   log, redact, rotate, query
    config/                  paths, layering
    daemon/
    cli/
  eval/
    dataset/                 JSONL cases (+ README on labeling protocol)
    runner/                  metrics, reports
    tools/                   generation/labeling helpers (Python ok)
  testdata/fixtures/{claude-code,codex}/<agent-version>/
  scripts/                   install scripts, canary
  .github/workflows/
```

---

## 8. Open questions and risks (ordered by potential damage)

1. **The enforcement boundary is leaky, and a false sense of safety is the worst outcome.** Fail-open on timeout/crash/bad output (Claude), `ask` unsupported and fail-on-error continues (Codex), `@` references and hosted tools bypass hooks, and obfuscated shell can evade any parser. *Mitigation:* explicit threat model doc, honest marketing ("guardrail, not sandbox"), `doctor` and heartbeat to prove the hook is live, native-config recommendations, adversarial eval set. *Decision needed:* what do we claim, and do we offer an opt-in "strict" mode that denies on any internal uncertainty?
2. **False-positive fatigue kills adoption** (users disable the hook or bypass modes). Observe-mode-first onboarding, per-category thresholds, measured false-interrupt rate as a release gate, easy `wardent allow`.
3. **Laya's real capability and latency are unknown to me.** If it cannot beat rules-only on Scope/Verify with calibrated confidence under ~300 ms, the product is still valuable as a deterministic guard, but the roadmap and pitch change. *Needs:* model facts (section "Questions") and an early eval.
4. **Adapter fragility.** Both agents evolve quickly (new hook events, fields and fixes every few releases; Claude docs reference versions such as v2.1.274; Codex hook format changed from `codex_hooks` to `hooks`). A competitor or the vendors themselves may absorb features. *Mitigation:* contract tests plus nightly canary, tolerant parsing, minimal reliance on unstable data (e.g., not the transcript format).
5. **Differentiation versus native features.** Claude Code has permission rules, an auto-mode classifier, sandboxing, `/goal`; Codex has sandbox/approval policy and `PermissionRequest`. Wardent must be clearly additive: cross-agent policy, scope awareness, verification, audit, explainability. Needs competitive review before pricing.
6. **Audit log is itself a sensitive artifact.** Commands and prompts routinely contain secrets. Redact before persisting, restrict file ACLs, size/rotation, and never export by default. Raw payload capture is debug-only and opt-in.
7. **Router has no enforcement path.** Neither agent exposes hook-level model/effort control. It will be advisory + launcher. Launcher flags are now verified for Claude (`--model`, `--effort`, `CLAUDE_CODE_EFFORT_LEVEL`) and for Codex via `-c model=... -c model_reasoning_effort=...`. Value is still weaker than the other capabilities. Out of release 1.
13. **(v0.2) Repos can disable our hooks** (`disableAllHooks` in project settings, `--bare`, `--settings`). Detect via heartbeat and `doctor`, tell the user, and recommend managed settings for teams later. Do not pretend we can prevent it.
14. **(v0.2) Claude Code auto mode (default) already covers much of generic risk detection.** Risk Guard must win on explainability, cross-agent policy, repo-specific rules, auditability and Windows coverage, not on "we also block rm -rf". See section 11.
8. **Windows reality.** Path separators, PowerShell tool, cmd quoting, named pipes, Codex-on-Windows hook support unverified, code signing, AV false positives on a tool that inspects commands. Needs its own spike and CI from day one.
9. **Trust and install friction on Codex** (hash-based re-review on any change, project hooks only when trusted). Stable command string and clear first-run guidance.
10. **Stop-hook verification can annoy or burn tokens.** Continuations cost money and can loop. Conservative defaults (max 2 forced continuations), only on explicit "done" claims.
11. **Shared-repo policy as an attack vector.** A malicious `.wardent/policy.toml` in a cloned repo could weaken protections or add noisy rules. Hence the "tighten only" merge rule and a user-layer trust prompt for repo policies.
12. **Licensing/commercial:** model license for Laya redistribution, telemetry stance (none by default), code-signing cost, enterprise needs (managed policy, central log export) possibly out of scope for v1.

---

## 9. Phased roadmap (each phase shippable on its own)

> **v0.2 scope:** the **first release is Phases 0, 1 and 2** (public release after Phase 2; Phase 1 can ship earlier as an observe-only beta). **Phases 3 to 6 are the later roadmap** and will be re-planned after release-1 feedback and eval data. Nothing in release 1 depends on Laya, Codex, or the Router.

**Phase 0: Spike and decisions (internal, no user release)**
Fully specified in `docs/PHASE0_SPIKE_PLAN.md`: measured cold start (Node vs Bun vs Go) on 3 OSes, empirical verification of hook failure semantics, real payload fixtures for both agents, Codex-on-Windows behaviour, install mechanics, shell-parsing feasibility, and the native-overlap measurement. Exit: language ADR from measured numbers, integration-contract ADR, fixtures, and a Phase 1 go/no-go memo.

**Phase 1: Observe and Audit (first adapter = whichever agent the project can verify; see section 1.6)**
`wardent install/uninstall/doctor`, first adapter (Claude Code or Codex), observe-only (never blocks), local audit log with redaction, `wardent log`, `wardent explain`. Value: a trustworthy, local session flight recorder and a validated install path on all 3 OSes. Also starts the dogfood corpus for the eval set.

**Phase 2: Risk Guard + Policy (Claude Code, deterministic only)**
`shellparse`, built-in risk and secret rules, protected paths, per-repo `policy.toml`, `warn/ask/deny` enforcement, fail-safe shim behaviour (section 5), `wardent allow`, first version of the eval set and CI gating for Risk. No model dependency. Value: standalone, useful safety product.

**Phase 3: Loop Detector + Result Verifier (deterministic)**
Session ledger, repeat-failure and oscillation detection, `Stop`-hook verification based on "was a test/build run after the last write, and did it pass?", continuation cap and user escalation. Eval strata for Loop/Verify. Value: catches stuck agents and unverified "done" with zero model.

**Phase 4: Provider interface + Laya + Scope Guard**
Provider interface, `NullProvider`, Laya sidecar provider, daemon, combination logic with confidence thresholds, Scope Guard (heuristic baseline, then model-assisted), semantic Risk/Verify enhancements. Gate: model must show lift over rules-only in the holdout. Value: semantic judgment, with the product still fully functional if the model is off.

**Phase 5: Codex adapter**
Parse/encode per section 2.4, `PermissionRequest` integration, trust-flow install UX, Codex fixtures and canary. Value: second agent with the same policy file.

**Phase 6: Model Router (advisory/launcher) and hardening**
`UserPromptSubmit` recommendation messages, `wardent run <agent>` launcher that applies recommended model/effort where the agent supports flags (after verifying them), router eval. Plus: managed policy layer, optional HTTP-hook/daemon fast path for Claude, signed installers/Homebrew/winget/Scoop, team features if in scope.

---

## 10. Decisions and remaining open items

**Resolved (v0.2):**
- First buyer: individual developers, self-serve, observe-first. Teams later (managed policy, shared audit export deferred).
- Failure stance: fail-safe prompting by default; deny only for critical hard rules; strict fail-closed is an opt-in mode.

**Phase 0 partial results (2026-10-04):** see `reports/PHASE0-STATUS.md`. Rubric outcome so far favors **Go** (measured on one Windows laptop only; agent-dependent experiments are BLOCKED). The "Claude Code first" assumption no longer holds (section 1.6).

**Still open:**
1. Laya repository link supplied (https://github.com/NandhaKishorM/laya.git); not reviewed yet (Phase 4). Model facts (size, format, license, latency) are still unknown to this document.
2. **Laya model facts were not provided** (size, runtime format, license/redistribution, latency, supported tasks, training status). Not needed for release 1. Required before Phase 4 planning: it decides in-process vs sidecar and which capabilities can be model-assisted. Please supply the model card when available.

---

## 11. Competitive differentiation vs native Claude Code and Codex controls

Honest starting point: the vendors have shipped substantial native safety. Wardent must not be positioned as "another command blocker".

### 11.1 What the native systems do (from the docs read)

| Area | Claude Code | Codex |
| :- | :- | :- |
| Permission model | Modes (`default`/Manual, `acceptEdits`, `plan`, `auto`, `dontAsk`, `bypassPermissions`); allow/ask/deny rules with deny > ask > allow; enforced by the harness, not the model | `approval_policy` (`on-request`, `never`, `granular`); permission profiles; command `rules` / `execpolicy` |
| Automatic review | **Auto mode (now the built-in default)**: a classifier model reviews actions; blocks scope escalation, unrecognized infrastructure, hostile-content-driven actions, force pushes to protected targets, destructive git, secret exfiltration, unresolved-variable `rm -rf`; reviews inter-agent messages; server-side review option | `auto_review`/guardian policy config exists in config reference (**[UNVERIFIED]** details; not read in depth) |
| OS isolation | Bash/PowerShell sandbox (Seatbelt, bubblewrap): filesystem and network boundaries. **macOS, Linux, WSL2 only; not native Windows** | Native sandbox on macOS/Linux and **native Windows** (`elevated`/`unelevated`), network off by default in sandboxed modes |
| Protected paths | Writes to `.git`, `.claude` etc. not auto-approved (except bypass) | Protected paths in writable roots |
| Admin controls | Managed settings, `allowManagedHooksOnly`, org model/effort caps | `requirements.toml`, managed hooks, MDM |
| Extensibility | Hooks (and `/goal`, a built-in Stop-hook shortcut for "keep going until condition") | Hooks, with trust review |

### 11.2 Where native controls are weak or absent (Wardent's opening)

1. **Cross-agent, one policy.** Native controls are per-vendor and in different formats (JSON settings vs TOML + rules). A developer using both Claude Code and Codex (common) maintains two systems. Wardent: one repo policy, one audit log, one vocabulary.
2. **Repo-specific, declarative, reviewable rules.** Native rules match tools and command patterns. They don't express "in this repo, `infra/prod/**` is never touched, migrations need a review ask, deploys only to staging". Wardent policy is per-repo, committed, diffable, and shows `explain` output.
3. **Explainable and auditable decisions.** Auto mode's classifier decision is a model verdict; the user sees a block, not an evidence trail. Wardent logs evidence + rule id + reason for every decision locally, queryable with `wardent explain`. Neither vendor offers a vendor-neutral, local, exportable audit trail (Codex's own docs note MCP hooks "do not provide a complete Compliance API audit trail").
4. **Scope Guard (intent vs edit).** Native systems ask "is this action safe/authorized", not "is this edit related to what I asked". Auto mode partially judges "escalates beyond your request" for actions, but not file-level scope drift on benign edits. (Later phase; not release 1.)
5. **Result verification and loop detection.** Native: `/goal` is the nearest feature. No deterministic "claimed done without a passing test after the last edit" check, no stuck-loop detection. (Later phase.)
6. **Native Windows for Claude Code users.** No Claude sandbox on native Windows; Wardent's deterministic command checks are an additional layer there. Must validate the PowerShell/cmd parsing quality, since this is a real selling point if it works.
7. **Determinism and latency.** Classifier-based review costs tokens and time and can be unavailable (the docs describe "auto mode cannot determine the safety of an action" failure). A local rules layer is predictable, free, and offline. It also works in `bypassPermissions`/`dontAsk` setups where users have removed prompts, **but** note deny rules and hooks still run there, which is exactly where users most need an independent guard.
8. **Local-first privacy of the safety layer itself.** Native auto mode sends actions to a model service. Wardent's tier 0 never leaves the machine.

### 11.3 Where Wardent loses or is redundant (be honest in positioning)

- Generic "block `rm -rf /`, force push, `git reset --hard`" is already covered by Claude auto mode and Codex sandbox/approvals for users who keep them on. A rules-only Risk Guard is **not differentiated on these alone**.
- OS-level isolation (filesystem/network containment) is strictly stronger than hooks and is the vendors' turf. Wardent should recommend it, not compete.
- Vendors move fast and can absorb features; any single capability is a feature, not a moat.

### 11.4 Positioning and implications for release 1

- **Pitch:** "One local policy and flight recorder for all your coding agents: explains what the agent did and why Wardent stopped it. Guardrail, not sandbox."
- Release 1 differentiators must therefore be: (a) **observe-first flight recorder** with `wardent log/explain` that works on day one with zero configuration (Phase 1), (b) **repo policy file** with protected paths, custom deny/ask rules and per-category thresholds (Phase 2), (c) **explanation for every decision**, (d) **secret-exposure and sensitive-file detection** beyond what generic classifiers do (e.g. custom secret patterns, `.env` reads via shell, tokens in prompts), (e) **works alongside** native controls (never emits `allow`, never loosens native rules).
- **Validation required in Phase 0/1 (not assumed):** measure, on a labeled set, what fraction of Wardent-only catches are *not* already blocked by Claude auto mode / Codex defaults. If overlap is ~100% on generic risks, Phase 2 must lean on repo policy, secrets, audit, and Windows rather than generic command rules. This is an explicit go/no-go input (see `docs/PHASE0_SPIKE_PLAN.md`, experiment E9).
- **Devin CLI is now the target; see 11.6.**
- **Interop rule:** Wardent's decisions compose with native ones. Claude: hook `deny` always wins; hook `ask` forces a prompt even in auto mode; Wardent abstains otherwise. Codex: `deny` and `PermissionRequest` only.

### 11.6 Differentiation vs Devin CLI native controls (release-1 target; docs-derived, UNVERIFIED)

What Devin ships natively: permission modes (Normal, Accept Edits, **Smart**, Bypass, Autonomous), deny > ask > allow rules, organization deny/ask rules that override user modes (Team Settings), hooks, and an OS sandbox (`--sandbox`) that is **not available on Windows** (the CLI hard-fails there rather than run unsandboxed).

Where Wardent can add value:
1. **Windows users have no sandbox.** Native containment is absent; an independent local audit trail plus (later) deterministic rules matter more there. Dev machine is Windows, so this is also the segment we can test.
2. **Bypass and Accept Edits users** get no review of shell commands (Bypass auto-approves everything). Observe-first Wardent gives them a flight recorder; Phase 2 rules give a guard that works even when prompts are off.
3. **Smart mode is a model judge with a fixed blocklist** (installs, mutating git, `rm`/`sudo`, destructive cloud CLIs, dotenv/key/git-config access always prompt). Generic risky-command blocking is therefore **already covered** in Smart mode and, by prompting, in Normal mode. Wardent must not claim generic command blocking as its differentiator; repo-specific rules, secret/sensitive-file exposure in outputs, and explainable audit are the pitch.
4. **Audit:** Devin exposes OTel logs (`tool_decision`, `tool_result`, correlation by `session.id`/`prompt.id`/`tool_use_id`) but its own docs note a decision source "does not always distinguish a hook approval from a user approval". Wardent's local, redacted, no-network JSONL log is complementary, not duplicative, and meets the local-first constraint (OTel needs a collector).
5. **Hook semantics are fail-open** (non-2 exit codes do not block); Phase 2 must convert internal failures into explicit decisions (not in Phase 1, which never blocks).

Open validation (experiments D1 to D6): payload shapes, timeouts, Windows hook shell, whether projects can disable hooks, and what Smart/Normal mode already stops on a labeled command set (the E9 equivalent).
