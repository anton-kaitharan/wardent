# E10: Shell-parsing feasibility (RUN, pilot-sized corpus)

Code: `spike/shellparse/` (throwaway). Raw: `spike/results/E10/summary.json`.

## Method
- Corpus: **86 POSIX commands** (categories: basic 24, danger 19, wrappers 13, substitution 6, heredoc 3, obfuscation 13, syntax edge 8), **46 PowerShell**, **12 cmd.exe**. This is **below the plan's 200 hand-labeled**; treat as a pilot.
- Task: extract the set of executed command names (wrappers like `sudo/env/timeout/xargs` and `bash -c '...'` unwrapped, `find -exec` followed, dynamic names reported as `<dyn>`).
- POSIX labels are hand-written; ground truth for syntax validity is `bash -n` (Git Bash). PowerShell ground truth is the **Windows PowerShell 5.1 parser** (CommandAst + alias resolution), which cannot be embedded in the product; a heuristic tokenizer was measured against it.
- Candidates: Go `mvdan.cc/sh/v3` v3.14.1 (released 2026-09-06); Node `shell-quote` 1.8.3 tokenizer; Node `web-tree-sitter` 0.25.10 + `tree-sitter-wasms` 0.1.13 bash grammar. Same wrapper-unwrapping logic ported to each.
- Metrics: exact set match; "flagged" mismatch (parse failure or `<dyn>` reported, i.e. conservative); **silent** mismatch (wrong and not flagged: the dangerous kind).

## Results (POSIX, 86 commands)
| Parser | Parses all 83 valid commands | Flags the 3 invalid | Exact | Flagged mismatch | Silent mismatch | Realistic categories* | p50 / max parse time |
| :- | :- | :- | :- | :- | :- | :- | :- |
| Go mvdan/sh | 83/83 | 3/3 | 81/86 (94%) | 4 | **1** | 65/65 | <1 us / 0.57 ms |
| Node tree-sitter-bash | 83/83 | 3/3 | 82/86 (95%) | 3 | **1** | 64/65 | 0.22 ms / 22 ms (first parse) |
| Node shell-quote (tokenizer) | 83/83 | 0/3 | 71/86 (83%) | 2 | **13** | 56/65 | 0.04 ms / 1.9 ms |
*basic + danger + wrappers + substitution + heredoc.

- `shell-quote` is a tokenizer: it does not see `$(...)`, backticks or `<(...)` (0/6 on substitution), mistakes keywords for commands, and accepts invalid syntax. Not viable for deterministic rules.
- Silent misses of the two real parsers: Go reports `\rm` as the literal `\rm` (alias-bypass spelling; trivially fixable by stripping a leading backslash). tree-sitter drops `[` test commands (benign). Both handle `$(...)`, backticks inside double quotes, heredocs, `bash -c`, `eval` flagging, `$CMD`/IFS tricks (reported `<dyn>` or flagged).
- **Label corrections after seeing results (disclosure):** two of my hand labels were wrong and I fixed them before the final numbers: (1) backticks inside double quotes in `git commit -m "... `rm -rf` ..."` really execute `rm` in bash, so the parsers were right; (2) `export` is a declaration, not a command. Numbers above use the corrected labels.
- Remaining mismatches are obfuscation that static analysis cannot resolve (`rm${IFS}-rf${IFS}/`, `$a -rf /`, `eval "$(... | base64 -d)"`). They are flagged as `<dyn>`, which is the intended conservative behavior (treat as at least medium risk).

## PowerShell and cmd
- PS 5.1 parser OK on 44/46 (the 2 failures are my intentional syntax errors).
- Heuristic tokenizer vs oracle: **41/46 exact, 1 flagged, 4 silent mismatches**. Of the silent ones, two are harmless extra tokens (`.count`, a URL), one is a real miss (`. ('i'+'ex') 'whoami'`: dynamic invocation through dot-sourcing), and one is a class of danger that command-name rules never see: **.NET static calls such as `[System.IO.Directory]::Delete('C:\x',$true)`** (the oracle also reports no command).
- cmd.exe heuristic: 12/12 on a 12-command set (too small to conclude).
- No mature PowerShell parser exists in either ecosystem that I could verify: Go has none; `tree-sitter-wasms` ships bash but **no powershell** grammar (I did not test a separately built tree-sitter-powershell). So **PowerShell is a language-neutral problem**: both Go and TS would need the same heuristic tokenizer plus conservative rules for `iex`, `-EncodedCommand`, `& $var`, dot-sourcing, `[Type]::Method` calls.

## Gate 3 verdict (pre-registered: >= 95% on the realistic subset, conservative fallback for the rest, viable PowerShell approach)
- POSIX: **PASS** for Go mvdan/sh (100% realistic, 94% overall) and tree-sitter (98.5% / 95%); **FAIL** for shell-quote.
- PowerShell: **Not disqualifying for any language**; heuristic accuracy 41/46 overall needs a larger corpus and a rules-level treatment of .NET calls.
- Caveat: the corpus is 86/46/12 instead of 200, and I wrote both the labels and the heuristics (author bias). The PS oracle is independent.
- Packaging note: tree-sitter in TS requires shipping wasm files and `web-tree-sitter`; I did not test it inside a Bun-compiled binary. Go's parser is a plain library link.
