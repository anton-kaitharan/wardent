# Phase 0 status (2026-10-04)

## Inputs received
- No Claude Code account. Windows dev machine. CI runners for macOS/Linux, numbers treated as relative.
- Cost cap: **left as the unfilled placeholder** `[your real number]`. Nothing I ran spends tokens or money, so none was needed. Anything that would (E3, E4, E9 empirical) stays blocked until you give a number.
- Codex access: **left as the unfilled conditional** `[If I have Codex: ...]`. `codex` is not installed on this machine and I did not assume access. Treated as **no Codex** until you say otherwise.
- Laya: https://github.com/NandhaKishorM/laya.git recorded; not opened (Phase 4 input).

## Experiment status
| Exp | Status | Evidence |
| :- | :- | :- |
| E1 Claude payload capture | **BLOCKED** (no Claude account) | |
| E2 Codex payload capture | **BLOCKED** (Codex access unconfirmed, not installed) | |
| E3 Claude failure semantics | **BLOCKED** (no Claude account) | |
| E4 Codex contract + trust | **BLOCKED** (Codex access unconfirmed). Note: you wrote "E2, E3, E7" for Codex; in the plan E3 is Claude's and E4 is Codex's contract/trust test | |
| E5 cold start | **RUN on Windows**; macOS/Linux workflow written, **NOT RUN** | `reports/E5-coldstart.md` |
| E6 in-agent overhead / daemon | **BLOCKED** (needs an agent) | |
| E7 Windows reality | **BLOCKED** for both agents. Partially answered without agents: Windows Defender first-run cost (in E5) | |
| E8 install/tamper | **config-logic part RUN**; the rest **BLOCKED** | `reports/E8-install-config-logic.md` |
| E9 native overlap | **BLOCKED** (needs agents). Static step (`claude auto-mode defaults`) also needs the Claude binary's account-free subcommand; not attempted | |
| E10 shell parsing | **RUN** (pilot-sized corpus) | `reports/E10-shellparse.md` |
| E11 distribution | **PARTIAL**: sizes + AV measured, signing researched; install flows NOT RUN | `reports/E11-distribution.md` |

## D1 (Go vs TypeScript) by the pre-registered rubric
- Gate 1a (warm latency): Go PASS, Bun-compiled TS PASS, Node FAIL.
- Gate 1b (first run after fresh download): everything native FAILS (Defender ~0.6 s); gate is non-discriminating. I did not change it; amendment proposed below.
- Gate 1d (in-agent): not measurable without an agent.
- Gate 2: Go PASS, Bun-compiled PASS (86 MB), Node FAIL.
- Gate 3: Go PASS (mvdan/sh); TS tree-sitter PASS; PowerShell neutral.
- Gate 4: Go is ~2.3x faster at p95, far outside the 25% band in which familiarity may decide.
**Rubric outcome: Go.** Bun-compiled TypeScript is a viable but slower and 34x larger runner-up. Confidence is limited: one older Windows laptop, no macOS/Linux data, no in-agent data, noisy tails.

**Proposed rubric amendment (needs your OK):** replace gate 1b with "first run after install <= 2 s and `wardent install/upgrade` pre-warms the binary".

## Do not start Phase 1: not started. Nothing past this report was run.
