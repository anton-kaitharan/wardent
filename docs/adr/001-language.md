# ADR-001: Implementation language for the Wardent runtime

Status: ACCEPTED (approved by the project owner on 2026-10-06 in reply "yes" to the Go proposal).

## Decision
Go, as a single static binary providing the CLI, hook shim and (later) daemon. TypeScript is retained for tooling only if needed.

## Evidence (all Windows, one older laptop; relative numbers; see `reports/E5-coldstart.md`, `reports/E10-shellparse.md`)
- Warm shim p95 (1 KB payload): Go 26 to 46 ms; Bun-compiled TypeScript 60 to 67 ms; Node 95 to 275 ms (an empty Node script already has p50 ~75-86 ms). Pre-registered gate: p95 <= 80 ms.
- Binary size: Go 2.5 MB; Bun-compiled 86 MB; Node needs a runtime.
- POSIX shell parsing: Go `mvdan/sh` 100% on the realistic subset of the pilot corpus. PowerShell has no mature parser in either ecosystem (language-neutral).
- Defender first-run scan (~0.6 s) affects every native binary equally.

## Consequences / caveats
- macOS/Linux numbers and in-agent overhead are NOT measured (UNVERIFIED).
- The first-run gate was amended in spirit: installer must pre-warm the binary (pending formal amendment).
- Bun-compiled TypeScript remains the fallback if Go proves unworkable.
