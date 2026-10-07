import json, subprocess, os, re, sys, statistics, shlex
sys.path.insert(0, os.path.dirname(__file__))
from corpus_posix import C
from corpus_ps import PS, CMD

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "..", "results", "E10")
os.makedirs(OUT, exist_ok=True)
GITBASH = r"C:\Program Files\Git\bin\bash.exe"

def run_json(argv, inp):
    p = os.path.join(OUT, "_in.json")
    json.dump(inp, open(p, "w"))
    r = subprocess.run(argv + [p], capture_output=True, text=True, encoding="utf-8")
    if r.returncode != 0:
        return None, (r.stderr or r.stdout)[-500:]
    return json.loads(r.stdout), None

def score(name, expected, results, valid=None):
    n = len(expected); exact = parse_ok = safe = silent = 0; per = {}
    silent_ids = []
    for i, exp in enumerate(expected):
        r = results[i]; got = set(r["exes"] or []) if r else set(); e = set(exp[1])
        ok = bool(r and r["ok"])
        parse_ok += ok
        match = got == e
        cat = exp[0]; per.setdefault(cat, [0, 0]); per[cat][1] += 1
        if match: exact += 1; per[cat][0] += 1
        elif (not ok) or ("<dyn>" in got and "<dyn>" not in e): safe += 1   # mismatch but flagged (parse failure or dynamic)
        else: silent += 1; silent_ids.append((i, exp[2], sorted(got)))
    micros = [r["micros"] for r in results if r]
    return dict(name=name, n=n, parse_ok=parse_ok, exact=exact, flagged_mismatch=safe, silent_mismatch=silent,
                per_cat={k: f"{a}/{b}" for k, (a, b) in per.items()},
                p50_us=statistics.median(micros) if micros else None, max_us=max(micros) if micros else None, silent_examples=silent_ids[:12])

# ---------- POSIX
posix_items = [dict(id=i, cmd=c[1]) for i, c in enumerate(C)]
posix_expected = [(c[0], c[2], c[1]) for c in C]
# bash -n ground truth for syntax validity
valid = []
for it in posix_items:
    r = subprocess.run([GITBASH, "-n"], input=it["cmd"], capture_output=True, text=True)
    valid.append(r.returncode == 0)
print(f"POSIX corpus: {len(C)} commands; bash -n says {sum(valid)} syntactically valid, {len(C)-sum(valid)} invalid (intentional syntax-error cases)")
res = {}
for label, argv in [("go-mvdan-sh", [os.path.join(HERE, "shellparse-go.exe")]),
                    ("node-shell-quote", ["node", os.path.join(HERE, "node", "extract.js")]),
                    ("node-tree-sitter-bash", ["node", os.path.join(HERE, "node", "extract.js")])]:
    mode = {"node-shell-quote": ["shell-quote"], "node-tree-sitter-bash": ["tree-sitter"]}.get(label, [])
    if mode:
        p = os.path.join(OUT, "_in.json"); json.dump(posix_items, open(p, "w"))
        r = subprocess.run(argv + [p] + mode, capture_output=True, text=True, encoding="utf-8")
        out, err = (json.loads(r.stdout), None) if r.returncode == 0 else (None, (r.stderr or "")[-400:])
    else:
        out, err = run_json(argv, posix_items)
    if out is None:
        res[label] = dict(name=label, error=err); print(label, "ERROR:", err); continue
    s = score(label, posix_expected, out)
    # parse_ok only meaningful on valid ones
    s["parse_ok_on_valid"] = f"{sum(1 for i,o in enumerate(out) if valid[i] and o['ok'])}/{sum(valid)}"
    s["flagged_on_invalid"] = f"{sum(1 for i,o in enumerate(out) if (not valid[i]) and not o['ok'])}/{len(valid)-sum(valid)}"
    res[label] = s
    print(f"{label:24} exact {s['exact']}/{s['n']}  flagged-mismatch {s['flagged_mismatch']}  SILENT-mismatch {s['silent_mismatch']}  parse_ok_on_valid {s['parse_ok_on_valid']}  invalid-flagged {s['flagged_on_invalid']}  p50 {s['p50_us']:.0f}us max {s['max_us']:.0f}us")
    print("   per-category exact:", s["per_cat"])
    for ex in s["silent_examples"]: print("   SILENT:", ex)

# ---------- PowerShell: oracle = Windows PowerShell 5.1 parser
ps_items = [dict(id=i, cmd=c[1]) for i, c in enumerate(PS)]
ip = os.path.join(OUT, "_ps_in.json"); json.dump(ps_items, open(ip, "w"), ensure_ascii=False)
op = os.path.join(OUT, "_ps_oracle.json")
subprocess.run(["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", os.path.join(HERE, "ps_oracle.ps1"), "-In", ip, "-Out", op], check=True, capture_output=True)
oracle = json.load(open(op, encoding="utf-8-sig"))
if isinstance(oracle, dict): oracle = [oracle]
oracle_by = {o["id"]: o for o in oracle}

ALIAS = {"rm": "remove-item", "ri": "remove-item", "del": "remove-item", "erase": "remove-item", "rd": "remove-item", "rmdir": "remove-item",
         "ls": "get-childitem", "dir": "get-childitem", "gci": "get-childitem", "cat": "get-content", "gc": "get-content", "type": "get-content",
         "iex": "invoke-expression", "iwr": "invoke-webrequest", "curl": "invoke-webrequest", "wget": "invoke-webrequest", "irm": "invoke-restmethod",
         "cd": "set-location", "sl": "set-location", "chdir": "set-location", "cp": "copy-item", "copy": "copy-item", "echo": "write-output",
         "ps": "get-process", "where": "where-object", "?": "where-object", "select": "select-object", "foreach": "foreach-object", "%": "foreach-object",
         "kill": "stop-process", "mv": "move-item", "move": "move-item", "sls": "select-string", "icm": "invoke-command", "saps": "start-process", "start": "start-process"}
KW = {"if", "elseif", "else", "foreach", "for", "while", "do", "switch", "function", "param", "try", "catch", "finally", "return", "throw", "in", "until"}

def tokenize_ps(s):
    toks = []; i = 0; cur = ""; n = len(s)
    def push():
        nonlocal cur
        if cur != "": toks.append(("w", cur)); cur = ""
    while i < n:
        ch = s[i]
        if ch in "'\"":
            q = ch; j = i + 1
            while j < n and s[j] != q:
                if q == '"' and s[j] == "`": j += 1
                j += 1
            if j >= n: return None   # unterminated
            cur += s[i:j + 1]; i = j + 1; continue
        if ch == "@" and i + 1 < n and s[i + 1] in "'\"":      # here-string: skip to closing
            q = s[i + 1]; end = s.find(q + "@", i + 2)
            if end < 0: return None
            cur += s[i:end + 2]; i = end + 2; continue
        if ch == "`": cur += s[i:i + 2]; i += 2; continue
        if ch in ";|\n": push(); toks.append(("sep", ch)); i += 1; continue
        if ch == "&":
            if s[i:i + 2] == "&&": push(); toks.append(("sep", "&&")); i += 2; continue
            push(); toks.append(("amp", "&")); i += 1; continue
        if ch in "{}()": push(); toks.append(("sep", ch)); i += 1; continue
        if ch.isspace(): push(); i += 1; continue
        cur += ch; i += 1
    push(); return toks

def ps_heuristic(src, depth=0):
    toks = tokenize_ps(src)
    if toks is None: return None
    out = []; start = True; prefix = None
    for kind, t in toks:
        if kind == "sep": start = True; prefix = None; continue
        if kind == "amp": prefix = "&"; continue
        if not start: continue
        low = t.lower()
        if low in KW: continue
        if t == ".": prefix = "."; continue
        start = False
        if t.startswith("$") or t.startswith("[") or t.startswith("("):
            if prefix: out.append("<dyn>")
            prefix = None; continue
        name = t.strip("'\"")
        if prefix and (t.startswith("'") or t.startswith('"')): name = name
        name = ALIAS.get(name.lower(), name.lower())
        out.append(name); prefix = None
    return out

ps_rows = []; silent = flagged = exact = parsed = 0; sil_ex = []
oracle_parse_ok = 0
for i, (cat, cmd) in enumerate(PS):
    o = oracle_by[i]; got = ps_heuristic(cmd)
    e = set(x.lower() for x in (o["exes"] if isinstance(o["exes"], list) else [o["exes"]]) if x)
    oracle_parse_ok += bool(o["ok"])
    if got is None: flagged += 1; continue
    parsed += 1
    g = set(got)
    if g == e: exact += 1
    elif "<dyn>" in g and "<dyn>" not in e: flagged += 1
    else: silent += 1; sil_ex.append((cat, cmd[:70], sorted(e), sorted(g)))
print(f"\nPowerShell corpus: {len(PS)}; PS 5.1 oracle parse OK {oracle_parse_ok}/{len(PS)}")
print(f"  heuristic tokenizer vs oracle: exact {exact}/{len(PS)}, flagged/failed {flagged}, SILENT-mismatch {silent}")
for x in sil_ex: print("   SILENT:", x)
# cmd.exe heuristic vs hand labels
def cmd_heur(s, depth=0):
    parts = re.split(r"&&|\|\||[&|]", s); out = []
    for p in parts:
        t = p.strip().lstrip("@")
        if not t: continue
        w = re.split(r"\s+", t)[0].strip('"').lower()
        out.append(w)
    return out
cx = sum(1 for c in CMD if set(cmd_heur(c[1])) == set(c[2]))
print(f"cmd.exe corpus: {len(CMD)}; heuristic exact {cx}/{len(CMD)}")
json.dump(dict(posix=res, ps=dict(n=len(PS), oracle_parse_ok=oracle_parse_ok, heuristic_exact=exact, flagged=flagged, silent=silent, silent_examples=sil_ex), cmd=dict(n=len(CMD), exact=cx)), open(os.path.join(OUT, "summary.json"), "w"), indent=1)
