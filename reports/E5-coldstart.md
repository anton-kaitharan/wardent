# E5: Cold-start benchmark (Windows measured; macOS/Linux NOT RUN)

Status: RUN on Windows 11 only. CI workflow for macOS/Linux written (`.github/workflows/spike-bench.yml`) but **not executed** (no git remote exists in this workspace; pushing is your call). Numbers are **relative**, from one machine.

## Environment
- Windows 11 Home build 26200, Intel Core (Family 6 Model 158 = Coffee Lake, ASUS X571GT-class laptop), 12 GB RAM, Defender real-time protection ON (`RealTimeProtectionEnabled: True`, platform 4.18.26080.4).
- Node v22.18.0, Bun 1.4.2, Go 1.26.8, hyperfine 1.20.0 (`-N`, no shell, `--warmup 10`, 200 runs/cell). Rust and Deno were not tested (optional in the plan).
- Same workload in every candidate ("shim-lite"): read stdin JSON, parse, normalize path, 51 regexes vs command, 30 globs vs path (glob->regex compiled each run), 5 secret regexes vs content, 40 policy regexes, read 8 KB policy + 2 KB rules from disk, append one JSONL line, emit a decision JSON. Outputs were cross-checked byte-identical across all 6 candidates on a dangerous payload.
- Limits: policy is JSON (not TOML), no SQLite/state store, no fsync, no daemon. A real shim will do somewhat more work, so these are floors.

## Results (ms; 1 KB payload; three runs on separate occasions: idle#1 / busy / idle#2)

| Candidate | p50 | p95 | p99 |
| :- | :- | :- | :- |
| floor: `node noop.js` | 86 / 77 / 74 | 181 / 107 / 92 | 443 / 137 / 181 |
| floor: Go no-op | 17 / 18 / 15 | 21 / 33 / 17 | 32 / 62 / 19 |
| node plain script | 90 / 91 / 80 | 136 / 210 / 95 | 252 / 332 / 111 |
| node esbuild-bundled | 94 / 86 / 90 | 133 / 117 / 150 | 218 / 224 / 288 |
| node bundled + compile cache | 114 / 85 / 137 | 275 / 116 / 214 | 755 / 189 / 423 |
| bun script | 52 / 47 / 49 | 68 / 62 / 95 | 133 / 91 / 417 |
| **bun --compile binary (86 MB)** | 47 / 45 / 47 | 60 / 60 / 67 | 103 / 78 / 83 |
| **Go binary (2.5 MB)** | 20 / 20 / 21 | 26 / 32 / 46 | 54 / 42 / 96 |

1 MB `Write` payload (p95): node plain 118/133/118; node bundled 129/139/116; bun script 63/73/136; bun compiled 77/118/100; Go 51/98/82.

Raw data: `spike/results/E5/windows_idle_run1`, `windows_busy`, `windows_idle_run2` (hyperfine JSON per cell plus `summary.json`).

## First run of a freshly written unsigned binary (Defender ON; 12 fresh copies each)
| | 1st run median | 1st run max | 2nd/3rd run median |
| :- | :- | :- | :- |
| Go | 632 ms | 1600 ms | 49 / 50 ms |
| Bun compiled | 634 ms | 1120 ms | 122 / 126 ms |
(Timed from Python, so absolute values include ~30 ms process-spawn overhead; compare rows to each other.) Raw: `spike/results/E5/win_cold_fresh_binary.json`.

## Gate evaluation (rubric fixed in PHASE0_SPIKE_PLAN section 4 before measuring)
| Gate | Go | Bun-compiled TS | Node (any variant) |
| :- | :- | :- | :- |
| 1a warm 1 KB: p95 <= 80, p99 <= 150 | PASS (all 3 runs) | PASS (all 3 runs) | **FAIL** (p95 95 to 275; even the empty-script floor has p95 92 to 181) |
| 1b first run after fresh download p95 <= 400 | **FAIL** (median 632) | **FAIL** (median 634) | n/a (node.exe already trusted; the script isn't scanned the same way) |
| 1c 1 MB: p95 <= 150 | PASS | PASS | PASS (but fails 1a) |
| 1d in-agent added latency (E6) | NOT MEASURED (BLOCKED, needs an agent) | NOT MEASURED | NOT MEASURED |
| 2 one-command install, no prerequisite, <= 100 MB | PASS (2.5 MB) | PASS (86 MB) | FAIL (needs Node) |

## Findings and caveats (read these)
1. **Node cannot meet the latency gate on this machine.** Its empty-script floor already sits at p50 ~75-86 ms. Bundling and compile cache did not help (compile cache was worse in two of three runs).
2. **Bun-compiled and Go both pass the warm gates; Go is ~2.3x faster** at p50/p95 and 34x smaller. Bun is within budget but with thin headroom on this older laptop, and its 1 MB tail (p99 up to 247 ms) is worse.
3. **Gate 1b is failed by every native binary, so it doesn't discriminate.** Defender scans a new unsigned executable on first run (~0.6 s median, up to 1.6 s). It is a one-time cost per binary file, but every upgrade creates a new file. I did not change the gate after seeing this; I recommend amending it to "first run <= 2 s, and `wardent install`/`upgrade` pre-warms the binary" (your approval needed). Code signing does not by itself remove the scan.
4. **The "busy" condition did not hurt** (a single pegged core on a multi-core CPU). It is not a real contention test, and run-to-run noise (p99 varies 2-4x between runs) is larger than the busy effect. Treat p95/p99 as +-50% estimates; p50s were stable (+-10%).
5. **Not measured:** macOS, Linux, a 2-vCPU low-end profile, in-agent overhead (E6), Rust, Deno, and a daemon/HTTP-hook variant. A daemon would not reduce a Node shim's spawn cost (the shim itself is the Node process).
6. Incidental: the Claude Code binary installed here (v2.1.31) is a Bun-compiled executable (stack traces reference `B:/~BUN/root/claude.exe`). This is only an inference from a stack trace, but it shows a vendor shipping Bun-compiled CLIs, so Bun-compiled TS is a credible alternative to Go if you value TypeScript.
