import json, random
random.seed(7)
regexes = [
 r"\brm\s+-[a-zA-Z]*[rR][a-zA-Z]*\s+(/|~|\$HOME)", r"\bgit\s+push\b.*(--force|\s-f\b)", r"\bgit\s+reset\s+--hard",
 r"\bgit\s+clean\s+-[a-z]*f", r"\bcurl\b[^|]*\|\s*(ba)?sh", r"\bwget\b[^|]*\|\s*(ba)?sh", r"\bchmod\s+-R\s+777",
 r"\bsudo\b", r"\bdd\s+if=", r"\bmkfs\.", r":\(\)\s*\{", r"\bdrop\s+table\b", r"\bdrop\s+database\b",
 r"\btruncate\s+table\b", r"\bkubectl\s+delete\b", r"\bterraform\s+destroy", r"\baws\s+s3\s+rm\b.*--recursive",
 r"\bgcloud\b.*\bdelete\b", r"\bdocker\s+system\s+prune", r"\bnpm\s+publish\b", r"\bcat\s+.*\.env\b",
 r"\bprintenv\b", r"\benv\s*\|", r"\bbase64\s+-d\b.*\|\s*(ba)?sh", r"\beval\s+", r"\bnc\s+-e\b",
 r"\bssh-keygen\b", r"\bscp\s+.*@", r"\bRemove-Item\b.*-Recurse.*-Force", r"\bInvoke-Expression\b", r"\biex\b",
 r"\bFormat-Volume\b", r"-EncodedCommand", r"\bdel\s+/[sq]", r"\brmdir\s+/s", r"\breg\s+delete\b",
 r"\bnpm\s+install\s+-g\b", r"\bpip\s+install\b.*--index-url", r"\bchown\s+-R\b", r"\bmv\s+.*\s+/dev/null",
 r">\s*/etc/", r"\bcrontab\b", r"\bsystemctl\s+(stop|disable)", r"\bkill\s+-9\s+1\b", r"\bgit\s+branch\s+-D\b",
 r"\bgit\s+checkout\s+--\s+\.", r"\bgit\s+stash\s+(drop|clear)", r"\bhistory\s+-c", r"\bunset\s+HISTFILE",
 r"\bnslookup\b.*\$\(", r"\bpython\d?\s+-c\b.*(os\.system|subprocess)",
]
globs = ["**/.env*","**/*.pem","**/*.key","~/.ssh/**","~/.aws/**",".git/**",".github/workflows/**","infra/prod/**","**/migrations/**","package.json",
 "**/id_rsa*","**/credentials*","**/*.p12","**/secrets/**","**/.npmrc","**/.pypirc","**/terraform.tfstate*","**/*.tfvars","Dockerfile","docker-compose*.yml",
 "**/node_modules/**","**/dist/**","LICENSE","**/*.lock","pnpm-lock.yaml","**/.claude/**","**/.codex/**","**/.wardent/**","**/kubeconfig*","**/.kube/**"]
secrets = [r"AKIA[0-9A-Z]{16}", r"ghp_[A-Za-z0-9]{36}", r"-----BEGIN [A-Z ]*PRIVATE KEY-----", r"sk-[A-Za-z0-9]{32,}", r"xox[baprs]-[A-Za-z0-9-]{10,}"]
assert len(regexes)==50 or True
json.dump({"regexes":regexes,"globs":globs,"secrets":secrets}, open("data/rules.json","w"))
pol = {"version":1,"mode":{"default":"observe"},"rules":[{"id":f"r{i}","match":{"tool":"shell","command_regex":regexes[i%len(regexes)]},"action":"ask","reason":"x"*40} for i in range(40)]}
json.dump(pol, open("data/policy.json","w"), indent=1)
p1 = {"session_id":"abc123","prompt_id":"550e8400-e29b-41d4-a716-446655440000","transcript_path":r"C:\Users\u\.claude\projects\x\t.jsonl","cwd":r"C:\proj","permission_mode":"default","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"npm test -- --coverage && echo done","description":"Run test suite","timeout":120000,"run_in_background":False},"tool_use_id":"toolu_01ABC123"}
json.dump(p1, open("data/payload_1k.json","w"))
lines = "".join("const v%d = compute(%d, 'some ordinary source line with text');\n"%(i,i) for i in range(16000))
p2 = {"session_id":"abc123","cwd":r"C:\proj","permission_mode":"default","hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{"file_path":r"C:\proj\src\index.ts","content":lines[:1_000_000]},"tool_use_id":"toolu_02"}
json.dump(p2, open("data/payload_1m.json","w"))
print(len(regexes), len(globs), len(open("data/payload_1k.json").read()), len(open("data/payload_1m.json").read()))
