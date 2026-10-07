# Wardent Phase 0: Spike Plan

Status: APPROVED; PARTIALLY EXECUTED on 2026-10-04 (E5, E10, E8 config-logic, E11 partial). Everything needing an agent is BLOCKED. See `reports/PHASE0-STATUS.md`. Companion to `docs/DESIGN.md` v0.2.
Nothing here is product code. Spike code is **throwaway**, lives only in `spike/`, and is never imported by product code. Only the *findings* (reports, fixtures, ADRs) carry forward.

## 0. Purpose and exit decisions

Phase 0 exists to turn the design's assumptions into measured or observed facts, and to make four decisions:

| # | Decision | Decided by | Output |
| :- | :- | :- | :- |
| D1 | **Go vs TypeScript** for the runtime | E5, E6, E10, E11 (measured numbers + rubric in section 4) | ADR-001 |
| D2 | **Integration contract**: exact hook config, command string, fail-safe shim rules, per-agent encoding | E1 to E4, E7, E8 | ADR-002 + verified capability matrix |
| D3 | **Phase 1 go/no-go** and what release-1 differentiation leans on | E9 (native overlap) + all | Go/no-go memo |
| D4 | Phase 1 and 2 scope adjustments (what is cut or added) | All | Updated roadmap |

Out of scope: any Laya work (details not supplied), Scope/Loop/Verify/Router, daemon design beyond what E6 needs for the language decision, and the Codex adapter beyond verification (Codex is release-2 per the design, but its contract is verified now because it shapes the core schemas).

## 1. Ground rules

1. **Pin and record versions.** Every report states exact versions of Claude Code, Codex CLI, Node, Bun, Go, `hyperfine`, OS build. Re-running on a newer agent version is an explicit step, not an accident.
2. **Disposable environment for anything that could do damage.** Experiments that run agents against dangerous-looking commands (E3, E4, E9) run in a throwaway VM or container with a temp repo and decoy files. Commands target only decoy paths inside that sandbox. No real credentials, no real remotes. Use fake secrets (obviously fake patterns such as `AKIAFAKE...`).
3. **Observe before concluding.** The docs are the hypothesis; the experiment is the evidence. Where docs and behaviour disagree, behaviour wins and the discrepancy is recorded in the report.
4. **Raw data is kept.** CSVs, payload captures and logs go into `spike/results/<experiment>/<os>/<date>/` with a `README` of the exact command and environment. Reports link to raw data.
5. **No telemetry, nothing leaves the machine** except the agent's own traffic to its vendor (needed for headless runs) and package/download traffic.
6. **Redact captured payloads before committing.** Fixtures are scrubbed of usernames, paths, tokens, and session ids (replace with stable placeholders), with the scrub script's rules documented.

## 2. Test environment matrix

| Env | Purpose | Notes |
| :- | :- | :- |
| Windows 11 x64 (native, not WSL), Defender real-time protection ON | Primary Windows target, AV effect on unsigned exes | The user's current OS; both native Claude Code and Codex |
| Windows 11 + WSL2 (Ubuntu) | Claude sandbox applicability, path translation | Secondary |
| macOS arm64 (recent) | Primary macOS | **Access needed; see open dependency below** |
| Ubuntu 22.04/24.04 x64 | Linux | VM or container |
| Low-end profile | Worst case | 2 vCPU / 4 GB VM, plus a "busy machine" condition (one core pegged by a CPU burner) |

Account/credential needs: a Claude Code login and a Codex login capable of headless runs (`claude -p`, `codex exec`). Token budget for E3, E4, E9 to be agreed (small; see E9).

**Open dependency:** I only have the Windows machine in this workspace. macOS and Linux runs need either CI runners (GitHub Actions `macos-latest`, `ubuntu-latest`, `windows-latest`) or machines you provide. Plan: benchmarks (E5, E6, E10, E11) run in CI on all three plus locally on the real Windows box; agent-in-the-loop experiments (E1 to E4, E7, E8, E9) that need an interactive login run on whichever machines you can supply, with Windows first. Anything not run on an OS is marked "not measured" in the report, not extrapolated.

## 3. Experiments

Each experiment lists: question, method, measurements, pass/fail, deliverable.

### E1. Claude Code: real hook payload capture

- **Question:** What exactly does each hook event deliver on each OS, and does it match the docs?
- **Method:** Install a capture-only hook (logs stdin JSON to a file, exits 0, emits nothing) for events: `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `Stop`, `SessionEnd`, `PermissionRequest`, `ConfigChange`, `SubagentStart/Stop`. Drive a scripted session (interactive and `claude -p`) covering: Bash command, PowerShell tool (Windows), Read, Write, Edit, MultiEdit-style edits, Glob/Grep, WebFetch, an MCP tool, a subagent call, a denied-by-native-rule call, a failing command, a `cd`, a file created via Bash redirect, and a prompt with an `@file` reference.
- **Measure:** Per event: field set, types, path formats (backslashes, drive letters, `~` expansion), presence of `permission_mode`, `effort`, `model`, `agent_id`; whether `PostToolUse` carries `tool_response` content and how large; `bashEditDiff` presence/shape; ordering of events; whether `@file` triggers any hook (docs say no).
- **Pass:** Fixtures captured for every event and tool class on Windows and at least one POSIX OS. All fields the design's `AgentEvent` needs (section 3.1) are available or a documented workaround exists.
- **Fail/escalate:** Any `AgentEvent` field not obtainable (e.g., no reliable way to get changed files for Bash edits, no tool class for PowerShell). Record and propose schema change.
- **Deliverable:** `fixtures/claude-code/<version>/<os>/*.json` (scrubbed), `reports/E1-claude-payloads.md` with a diff against the docs.

### E2. Codex: real hook payload capture

- Same as E1 for Codex: `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PermissionRequest`, `PostToolUse`, `Stop`, `SessionEnd`; tools: shell, `apply_patch` (check how `tool_input.command` encodes the patch and file paths), MCP tool, a hosted tool (`WebSearch`) to confirm no hook fires, `write_stdin` poll.
- **Measure:** field sets, `turn_id`, `model`, patch text format, path style on Windows, how `PermissionRequest` appears relative to `PreToolUse`, whether `PermissionRequest` fires under each `approval_policy` (`on-request`, `never`, `granular`).
- **Pass:** fixtures for each event on Windows and one POSIX OS; patch parsing feasibility confirmed (can we extract file paths and ops from `apply_patch` text deterministically?).
- **Fail/escalate:** patch text not parseable into file ops, or hosted-tool/other paths bypass hooks in ways beyond the documented ones.
- **Deliverable:** `fixtures/codex/<version>/<os>/*.json`, `reports/E2-codex-payloads.md`.

### E3. Claude Code: failure-semantics verification (fail-open traps)

- **Question:** Do the documented fail-open behaviours hold, and which failures can the shim convert into a block?
- **Method:** Matrix of hook behaviours against a `PreToolUse` Bash hook, each run against a "dangerous-looking" decoy command and observing whether the tool ran:

| Case | Expected per docs |
| :- | :- |
| exit 2 + stderr | blocks |
| exit 0 + JSON `deny` | blocks |
| exit 1 | proceeds (non-blocking) |
| exit 0 + invalid JSON | proceeds, error notice |
| exit 0 + schema-invalid JSON | proceeds, error notice |
| crash (uncaught exception / segfault-equivalent) | proceeds |
| script path missing / not executable | proceeds (127 notice) |
| hang beyond `timeout` | proceeds, hook cancelled |
| JSON `ask` | prompts, including under `auto` mode |
| JSON `allow` against a native `deny` rule | still denied |
| JSON `allow` against a native `ask` rule | still prompts |
| exit 0 + JSON `deny` under `bypassPermissions` | blocks? (verify) |
| hook output very large (>1 MB) | ? |
| two hooks, one `deny` one `allow` | deny wins |
| `disableAllHooks:true` in project settings vs `false` in user settings | project wins, hooks off (docs imply) |
| `--bare`, `--settings '{"disableAllHooks":true}'` | hooks off |
| `@file` reference to a path covered by a hook `Read` matcher | no hook fires |

- **Measure:** observed outcome per case; user-visible notice text; whether the session shows a warning that a security hook failed.
- **Pass:** every row observed and recorded. Specifically required: (a) confirm our shim's "catch-all then emit deny/ask on internal failure" strategy is expressible, i.e. a *top-level* try/catch can still reach exit 2 or JSON for each internal failure we can intercept; (b) the set of **un-interceptable** failures (path missing, interpreter missing, timeout, hooks disabled) is enumerated for `doctor`'s checks.
- **Fail/escalate:** any case where behaviour contradicts the docs in the unsafe direction (e.g., hook `allow` overrides a native deny). That forces a design change (section 0 item 4 of the design).
- **Deliverable:** `reports/E3-claude-failure-semantics.md` with the verified matrix; updated design section 5.

### E4. Codex: contract and trust verification

- **Method / matrix:** `deny` via JSON, legacy `decision:block`, exit 2; `permissionDecision:"ask"` (expect: hook marked failed, call proceeds); `continue:false` (expect: failed, proceeds); hook crash; hook timeout; invalid JSON; `Stop` hook returning JSON vs plain text; `Stop` `decision:block` loop (is there any cap? how many continuations before something intervenes?); `PermissionRequest` allow/deny/abstain; two hooks disagreeing.
- **Trust experiments (critical for the stable-command-string decision):**
  1. Install a hook, approve in `/hooks`. Change **only the script file contents** while leaving the command string identical. Is trust invalidated (i.e., is the hash over the definition only, or over file content)?
  2. Change one character of the command string. Confirm invalidation.
  3. Install at user level vs repo level; test the untrusted-project case (project hooks skipped).
  4. What does the user see when an untrusted hook is skipped? Is it loud or silent?
  5. `--dangerously-bypass-hook-trust` behaviour for scripted runs.
- **Pass:** all rows observed; trust-hash scope determined; Stop continuation cap determined (or confirmed absent).
- **Decision impact:** if trust covers only the command string, "stable command + evolving binary" is viable (design assumption). If it covers file content or the resolved binary, then upgrades re-prompt users and the install design must change (e.g., a never-changing tiny launcher script).
- **Deliverable:** `reports/E4-codex-contract-trust.md`.

### E5. Cold-start benchmark (primary input to D1)

- **Question:** What is the real per-invocation cost of the hook shim in each candidate runtime?
- **Candidates** (each implements the *same* throwaway workload, no product code reuse):
  - A. Node (current LTS), plain JS script
  - B. Node, single esbuild-bundled file (no `node_modules` resolution), plus V8 compile cache if supported
  - C. Bun, interpreted script
  - D. Bun `--compile` standalone binary (also record binary size)
  - E. Deno `compile` (optional, only if time allows)
  - F. Go static binary
  - G. Rust static binary (optional control, to see whether Go leaves meaningful headroom)
- **Workload ("shim-lite"):** read JSON from stdin (two sizes: ~1 KB typical `PreToolUse` Bash payload and ~1 MB `Write` content), parse it, normalize a path (Windows backslash to forward slash), evaluate ~50 compiled regexes and ~30 glob patterns against command/path, append one JSONL line to a log file (with flush; test with and without fsync), write a decision JSON to stdout, exit. Policy load: read a ~5 KB policy file from disk each invocation, then variant with mtime-cached parse (only possible with a daemon, so measured only in E6).
- **Method:** An external harness spawns each candidate N times and records wall-clock from spawn to exit. Use `hyperfine` (cross-platform) with `--warmup`, `--runs >= 200`, `--shell=none`, plus a custom spawner on Windows to avoid shell wrapper cost. Conditions per OS: (1) warm (repeated), (2) **cold** (first run after the binary was freshly built/downloaded, to capture Defender/Gatekeeper first-scan costs), (3) after cache drop where the OS allows, (4) with a CPU burner occupying one core, (5) low-end VM. Run on AC power, document power plan. Each condition run on at least two separate occasions to detect noise.
- **Also record:** peak RSS, binary/bundle size, and **variance** (p99 and count of runs > 200 ms), because a rare 400 ms stall on every hundredth tool call is felt more than a steady 40 ms.
- **Report:** p50 / p95 / p99 / max per candidate per OS per condition, as tables and distribution plots, plus the machine spec.
- **Deliverable:** `spike/bench/` harness (throwaway), `results/E5/*.csv`, `reports/E5-coldstart.md`.

### E6. In-agent overhead and daemon/HTTP alternatives

- **Question:** What does a user actually feel when Wardent sits in `PreToolUse`, and can a daemon or HTTP hook rescue a slower runtime?
- **Method (in-agent):** Scripted headless Claude Code runs (`claude -p`) with a fixed task producing ~30 sequential tool calls (reads, greps, bash echo), measured with (i) no hooks, (ii) capture-only hook per candidate runtime. Compute per-tool-call added latency from hook-side timestamps (`start`, `exit`) and total wall-clock difference across >= 10 repetitions. Repeat for Codex (`codex exec`) with its hook.
- **Method (daemon):** Prototype a minimal local server in the leading candidate runtime and a tiny client shim; measure (a) Claude **HTTP hook** to `127.0.0.1` (no process spawn), (b) command hook where the shim is a tiny native client calling the daemon, (c) behaviour when the daemon is down (connection refused path, timeout path), including what Claude does (docs: non-blocking error).
- **Key point to verify, not assume:** a daemon does **not** reduce spawn cost if the command-hook shim is itself a Node process. A daemon only helps if the shim is native or if the agent supports HTTP hooks (Claude yes, Codex no, per docs). So "TypeScript + daemon" must still pass E5's shim gate unless Claude-HTTP-only is accepted for release 1 (it is not: Codex is a stated target).
- **Pass/fail:** see decision rubric in section 4 (latency gates).
- **Deliverable:** `reports/E6-inagent-overhead.md`.

### E7. Windows reality check (both agents)

- **Questions:**
  1. Which shell does Claude Code use to run `command` hooks on native Windows (Git Bash, PowerShell, cmd)? How are quoting, environment variables (`${CLAUDE_PROJECT_DIR}`) and the `args: []` array form handled? Does a bare `wardent hook ...` resolve on PATH?
  2. What does the PowerShell tool deliver in `tool_input`? Quoting/encoding fidelity for `-EncodedCommand`, here-strings, backtick escapes.
  3. Codex on native Windows: do hooks fire at all under the `elevated` and `unelevated` sandboxes? Does the hook process run **inside** the sandbox or outside it? Which user/token? Is `commandWindows` honoured? Where is the `hooks.json` located (`%USERPROFILE%\.codex`)?
  4. Path forms: drive letters, UNC, 8.3 short names, case-insensitivity, `\\?\` prefixes, junctions/symlinks; do hooks see normalized or raw forms?
  5. Defender/SmartScreen behaviour on an unsigned Go/Bun binary invoked per tool call (first run delay, quarantine risk), versus Node script.
- **Pass:** each question answered with captured evidence; a documented list of Windows-specific adapter requirements; a yes/no answer on "Codex hooks work on native Windows" (if no, the Codex adapter is Windows-excluded or WSL-only, which changes release-2 claims).
- **Deliverable:** `reports/E7-windows.md`.

### E8. Install, config and tamper-detection mechanics

- **Questions and method:**
  1. **Claude settings precedence and hook merging:** confirm user/project/local/managed layers merge hooks; test idempotent insertion into an existing `~/.claude/settings.json` that already has user hooks, comments-free JSON, unusual key order, and a file with invalid JSON (installer must refuse, not clobber). Record managed-settings file locations per OS and whether `allowManagedHooksOnly` would silently kill user-level Wardent hooks (relevant to the later teams phase, and to individuals on managed machines).
  2. **Codex config locations:** `hooks.json` vs inline `[hooks]` in `config.toml`; behaviour when both exist (docs: merged with a warning); `features.hooks`; project `.codex/` trust state.
  3. **Uninstall cleanliness:** can we remove only our entries, leaving everything else byte-identical?
  4. **Heartbeat / tamper detection:** with Wardent installed, test what `SessionStart`, `ConfigChange`, and a periodic heartbeat can detect. Specifically: when a project sets `disableAllHooks:true`, or `--bare` is used, **no Wardent code runs**, so detection must be out-of-band: `wardent doctor` comparing recent agent activity (transcript mtimes / session store) against Wardent's last-event timestamp. Determine whether agent session artifacts are reliably readable for that (both vendors say transcript format is not stable, so only mtime/existence may be used).
  5. **Verified-path install:** does an absolute path survive upgrades/moves; does a PATH-resolved command work in GUI-launched agents (IDE extension, desktop app) that may have a different PATH than the terminal?
- **Pass:** installer algorithm specified (steps, backup, idempotency, rollback); list of silent-disable scenarios and which are detectable.
- **Deliverable:** `reports/E8-install-and-tamper.md`, input to ADR-002.

### E9. Native-overlap measurement (differentiation go/no-go input)

- **Question:** For the generic risks Wardent would block in release 1, what fraction do Claude Code (default `auto` mode and Manual mode) and Codex (default approval + sandbox) already stop or prompt on? What does Wardent catch that they do not?
- **Static step (cheap, first):** run `claude auto-mode defaults` (and `config`) and diff the built-in classifier rules against the draft Risk Guard category list (design section 4 and 8). Review Codex default command rules/approvals (read `agent-approvals-security` and `rules` pages, which are on the "still not read" list).
- **Empirical step:** build a **seed labeled set** (about 150 risky + 150 benign command/edit scenarios across categories: destructive fs, destructive git, curl|sh, secrets read/print/commit, protected-path writes, prod-infra commands, network exfil, obfuscated variants, Windows PowerShell equivalents). Run each as a scripted agent task in the disposable environment, in: Claude `auto`, Claude Manual/`default` with `-p` (what does it do headless?), Claude with sandbox on, Codex `workspace-write` + `on-request`. Record per scenario: ran / prompted / blocked, and by what mechanism. Run the same set through a **prototype baseline** (simple regex + path rules) to see what it would add.
- **Cost control:** use short single-step tasks; cap tokens; budget agreed up front; prefer cheaper models where the classifier behaviour is model-independent (note which results may depend on model).
- **Report:** a 3-way matrix: natively blocked / natively prompted / not covered. Highlight scenarios Wardent catches and natives miss, and false-positive behaviour of natives on the benign set.
- **Decision rule (input to D3, not an automatic gate):**
  - If natives already stop or prompt on >= 90% of the *critical generic* scenarios in their default configs, release-1 Risk Guard messaging and scope **must lean on** repo policy, secrets/sensitive-file detection, audit/explain, cross-agent policy, and Windows coverage, not generic command blocking.
  - If natives miss > 25% of critical scenarios in some default config (e.g., `bypassPermissions`, `dontAsk` with broad allow rules, Codex `never`), those configs become the target segment and the onboarding flow should detect and recommend Wardent there.
  - Either way, record the numbers; decide with the user.
- **Deliverable:** `eval/seed-dataset-v0/` (JSONL, labeled, shared format with the future eval set), `reports/E9-native-overlap.md`. The seed set seeds the Phase 2 eval.

### E10. Shell-parsing feasibility (affects language choice)

- **Question:** Can we parse real shell commands well enough for deterministic rules, in each candidate language, for POSIX shells **and** PowerShell/cmd?
- **Method:** Collect a corpus: (a) commands captured in E1/E2 and E9, (b) public command corpora/shell history samples that contain no private data, (c) adversarial/obfuscated forms (nested quoting, `$(...)`, backticks, here-docs, `eval`, `bash -c`, `sudo`, env prefixes, `xargs`, base64 pipes, `$IFS`, PowerShell aliases (`rm`, `del`, `iex`), `-EncodedCommand`, cmd `/c`). Evaluate candidate parsers: Go (`mvdan.cc/sh`), TypeScript options (a bash parser package, tree-sitter bindings), and for PowerShell whatever exists in each ecosystem (tree-sitter-powershell, heuristic tokenizer). Metrics: parse success rate, "conservative on failure" behaviour (unparseable + dangerous token must score as at least medium), correct extraction of the primary executable and target paths on a hand-labeled 200-command subset, parse time per command.
- **Pass:** a recommended parsing approach per dialect with measured success rate >= 95% on the realistic subset, and a defined conservative fallback for the rest. A language is **disqualified for release 1** only if it has no viable PowerShell approach at all (the heuristic tokenizer fallback counts as viable if it meets the labeled-subset accuracy).
- **Deliverable:** `reports/E10-shellparse.md`.

### E11. Distribution and one-command install

- **Question:** Can we deliver "installable in one command" on all three OSes for each candidate runtime, and what friction/warnings do users hit?
- **Method:** Publish a hello-world release (GitHub Releases, private/test repo) per candidate: Go binary, Bun-compiled binary, npm package (for Node variants). Simulate a fresh user on each OS: `curl | sh` (macOS/Linux), PowerShell one-liner and `winget`/`scoop` manifest (Windows), `npm i -g`, Homebrew tap (if feasible in spike). Record: steps, time, prerequisites, security prompts (SmartScreen, Gatekeeper quarantine xattr, Defender quarantine), PATH propagation to GUI-launched agents, binary size, and what code signing/notarization would be required to remove warnings (research the process and cost; do not purchase in Phase 0).
- **Pass:** for the chosen runtime, a documented install path per OS that runs with <= 1 command and no mandatory prerequisite beyond the OS; list of signing tasks with owners.
- **Deliverable:** `reports/E11-distribution.md`.

## 4. Decision rubric for D1 (Go vs TypeScript), defined before measuring

Numbers are initial thresholds. They are **fixed before E5 runs** so results can't be rationalized after the fact. They may be revised only with a written note before the benchmark is executed.

**Gate 1: Latency** (all must hold on every measured OS on the primary-laptop condition; "busy" condition tracked but not gating):
- Tier-0 shim (E5 workload, ~1 KB payload) wall time **p95 <= 80 ms** and **p99 <= 150 ms**, measured warm.
- **Cold first-run** after fresh download p95 <= 400 ms, and only once per binary install (Defender/Gatekeeper first-scan is a one-time cost; recurring scans per invocation would fail this gate).
- 1 MB payload p95 <= 150 ms.
- In-agent (E6): added latency per tool call p95 <= 100 ms; no run adds > 500 ms.
Rationale: a user perceives roughly 100 ms as noticeable; agent tool calls usually take far longer, but parallel batches and many sequential calls multiply the cost. Thresholds are hypotheses to be challenged by E6's in-agent data.

**Gate 2: Distribution:** a one-command install on all three OSes with **no required pre-installed runtime**. A TS candidate passes only as a single-file compiled binary (Bun/Deno), and only if it also passes Gate 1 and size is <= 100 MB. "Requires Node" fails this gate for the self-serve individual-developer audience (Node absence cannot be assumed, especially for Codex users).

**Gate 3: Parsing viability:** E10 passes for POSIX and PowerShell dialects.

**Gate 4 (tie-break, only among candidates that pass gates 1 to 3):** weighted score of: (a) latency p95 headroom, (b) install friction, (c) Windows AV behaviour, (d) cross-platform test/CI simplicity, (e) team familiarity. Familiarity is allowed to decide only a near tie (candidates within 25% on p95 latency).

**Outcomes:**
| Result | Decision |
| :- | :- |
| Only Go (and/or Rust) passes gates 1 to 3 | Go (Rust only if E5 shows it materially beats Go, e.g. >30% p95, which I do not expect to matter). TS reserved for tooling. |
| Go and a compiled-TS candidate both pass | Tie-break on gate 4. Default to Go unless the team strongly prefers TS and the Bun/Deno binary shows no worse Windows AV behaviour and install friction. |
| Only TS passes (Go fails an unexpected gate, e.g. no viable PowerShell parsing) | TypeScript, as a compiled binary; document the reason. |
| Nothing passes Gate 1 on Windows | Escalate: tier-0 must move to a tiny native launcher plus daemon, or accept looser budgets. Re-plan before Phase 1. |

## 5. Dependencies and ordering (no duration estimates)

```
Prereq: access to Windows (now), CI runners for 3 OSes, agent logins, token budget
   │
   ├─ Track A (agent facts): E1, E2 -> E3, E4 -> E7, E8
   ├─ Track B (language):    E5 -> E6 (needs E1 payload shapes for realistic workload), E10, E11
   └─ Track C (positioning): E9 static step (anytime) -> E9 empirical (needs E1/E2 to script agents; needs E10 corpus overlap)
                │
                └─> D1, D2, D3, D4 -> Phase 1 go/no-go memo
```
- E1/E2 first: their payloads define the realistic workload for E5/E6.
- E5 does not need agents and can start immediately and in parallel.
- E3/E4 before any adapter design is trusted.
- E9's empirical step is last (largest token cost, needs fixtures and parsed corpus).

## 6. Deliverables checklist

| Deliverable | Path | From |
| :- | :- | :- |
| Language ADR (with the raw numbers linked) | `docs/adr/001-language.md` | E5, E6, E10, E11 |
| Integration-contract ADR (hook command string, shim failure rules, per-agent encodings, install algorithm, tamper detection) | `docs/adr/002-integration-contract.md` | E1 to E4, E7, E8 |
| Verified capability matrix (replaces design section 1.3 "docs say" with "observed") | update `docs/DESIGN.md` | E1 to E4, E7 |
| Scrubbed hook payload fixtures, both agents, per OS/version | `fixtures/` | E1, E2 |
| Benchmark harness + raw CSVs + reports | `spike/bench/`, `results/`, `reports/E5`, `E6` | E5, E6 |
| Windows report | `reports/E7-windows.md` | E7 |
| Install/tamper report | `reports/E8-install-and-tamper.md` | E8 |
| Native-overlap report + seed labeled dataset v0 | `reports/E9-native-overlap.md`, `eval/seed-dataset-v0/` | E9 |
| Shell-parse report + corpus | `reports/E10-shellparse.md`, `eval/shell-corpus/` | E10 |
| Distribution report incl. signing tasks and costs | `reports/E11-distribution.md` | E11 |
| Updated risk register and roadmap | `docs/DESIGN.md` sections 8, 9 | all |
| **Phase 1 go/no-go memo** (one page: decisions D1 to D4, what changed vs the design, scope cuts, remaining risks) | `docs/adr/003-phase1-go-no-go.md` | all |
| Spike teardown: `spike/` marked throwaway; only reports, fixtures, datasets retained | repo | all |

## 7. Exit criteria for Phase 0

Phase 0 is complete when **all** hold:
1. D1 decided by applying the section 4 rubric to measured data on Windows plus at least one POSIX OS (CI-measured on all three where possible; missing OS clearly flagged).
2. E3 and E4 matrices fully observed; no unresolved contradiction with the design's `allow`-never rule (design section 0 item 4) and the fail-safe rules (section 5). If one exists, the design is amended and re-approved.
3. Fixtures exist for every `AgentEvent` source event the Phase 1 adapter needs (Claude: all of E1's list).
4. The Codex-on-native-Windows question has a yes/no answer with evidence.
5. E9 report delivered and its decision rule applied with the user.
6. ADR-001, ADR-002, ADR-003 written and approved by you.

**Stop/re-plan triggers (not merely failures):**
- Hook p95 overhead cannot meet Gate 1 in any candidate on Windows.
- A verified behaviour makes the "never emit `allow`" or "shim converts internal failure to explicit decision" strategy impossible.
- E9 shows natives cover essentially all generic risks **and** repo-policy/secrets/audit value can't be demonstrated in a prototype. In that case, the product thesis for release 1 needs rethinking before Phase 1.

## 8. Risks to the spike itself

| Risk | Mitigation |
| :- | :- |
| Benchmarks are noisy (AV, power plans, thermal) | Fixed protocol, >= 200 runs, repeated on separate occasions, report p99 and outlier counts, record environment |
| Docs/behaviour drift mid-spike (agents release often) | Pin versions; one scheduled re-run of E1/E3 smoke at the end |
| Token cost of E3/E4/E9 | Short scripted tasks, capped budget agreed in advance, deterministic decoys |
| Experiments touching real data | Disposable VM, decoy repo, fake secrets only |
| macOS/Linux access gaps | CI runners for benchmarks; "not measured" label otherwise |
| Over-building the spike into the product | `spike/` is explicitly throwaway; only findings and fixtures move forward |
| Unfilled Laya details | Not on Phase 0's critical path; flagged in `DESIGN.md` section 10 |

## 9. Questions I need answered before starting (blocking, minimal)

1. **Environments:** Can you provide a macOS machine (or are CI runners enough for the benchmarks, with agent-in-the-loop tests on Windows only for release 1)?
2. **Budget and accounts:** Are Claude Code and Codex logins available in this environment for headless runs, and what token/cost cap for E3, E4 and E9?
3. **Laya:** still waiting on the model card (not needed for Phase 0 or release 1, needed before Phase 4 planning).
