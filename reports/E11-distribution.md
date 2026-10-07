# E11: Distribution and one-command install (PARTIAL: measured sizes/AV + desk research; install flows NOT RUN)

## Not run
No GitHub remote exists and no release was published, so none of these were executed: `curl | sh`, PowerShell one-liner, `winget`/`scoop` manifest, Homebrew tap, `npm i -g`, and SmartScreen/Gatekeeper behaviour on a downloaded file. All three macOS/Linux/Windows install flows are **NOT MEASURED**.

## Measured locally (Windows)
| Artifact | Size | Needs runtime | Defender first run (median / max) |
| :- | :- | :- | :- |
| Go static binary | 2.5 MB | no | 632 ms / 1600 ms |
| Bun `--compile` binary | 86 MB | no | 634 ms / 1120 ms |
| Node bundled script | 1.1 KB (+ Node 22) | yes (Node) | n/a |
Implication: a self-serve installer should (a) run the binary once at install/upgrade to absorb the scan, (b) keep the hook command string stable.

## Desk research (sources fetched 2026-10-04)
- **Windows signing:** Microsoft's recommended service for non-Store distribution is Azure Artifact Signing (formerly Trusted Signing), about USD 9.99/month; **individual developers must be located in the USA or Canada** (organizations: USA, Canada, EU, UK and some other regions); requires a paid Azure subscription (no free/trial); identity validation takes business days; **SmartScreen reputation still builds over time even when signed**. Outside those regions, OV certificates are the alternative (cost/process not researched). Source: https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options and https://learn.microsoft.com/en-us/azure/artifact-signing/faq
- **macOS:** software signed with a Developer ID and distributed outside the App Store must be notarized (Apple Developer Program membership required; fee not re-verified here). Standalone binaries can be notarized but **tickets cannot be stapled to a bare executable**. Per an Apple engineer on the developer forums, `curl`/`scp` downloads do **not** set the `com.apple.quarantine` attribute, so a `curl | sh` install generally avoids Gatekeeper prompts, while browser downloads trigger them. Sources: https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution and https://developer.apple.com/forums/thread/706442
- Not researched: winget/Scoop submission requirements, Homebrew tap/core policy, Linux packaging, npm platform-package approach.

## Questions this raises for you
1. Where are you located (affects whether Azure Artifact Signing is available to you as an individual)?
2. Is an unsigned first release (with install-time pre-warm and a `curl`/PowerShell installer) acceptable for the observe-first beta?

## Verdict vs plan
Gate 2 (one-command, no runtime prerequisite) can only be asserted for Go and Bun-compiled binaries on size/runtime grounds; the actual install flows remain unverified.
