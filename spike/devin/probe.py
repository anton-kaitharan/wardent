"""THROWAWAY Phase 0 probe for Devin CLI hooks: records raw hook stdin + environment, and injects faults on request."""
import sys, os, json, time, subprocess, glob

LOG = os.path.join(os.path.dirname(os.path.abspath(__file__)), "log")
os.makedirs(LOG, exist_ok=True)
t0 = time.time()
raw = sys.stdin.buffer.read()
try:
    text = raw.decode("utf-8")
    ev = json.loads(text)
except Exception as e:
    text, ev = raw.decode("utf-8", "replace"), {"_parse_error": str(e)}

def parents():
    try:
        ps = ("$p=%d; $o=@(); for($i=0;$i -lt 6 -and $p;$i++){ $x=Get-CimInstance Win32_Process -Filter \"ProcessId=$p\"; if(!$x){break}; $o+=($x.Name+' | '+$x.CommandLine); $p=$x.ParentProcessId }; $o -join \"`n\"" % os.getpid())
        return subprocess.run(["powershell", "-NoProfile", "-Command", ps], capture_output=True, text=True, timeout=20).stdout.strip().splitlines()
    except Exception as e:
        return [f"ERR {e}"]

name = ev.get("hook_event_name")
rec = {
    "ts": t0, "argv": sys.argv, "event": name, "stdin_bytes": len(raw), "payload": ev,
    "env": {k: v for k, v in os.environ.items() if k.startswith(("DEVIN", "CLAUDE", "WINDSURF")) or k in ("ComSpec", "SHELL", "PSModulePath_present")},
    "cwd": os.getcwd(), "stdin_isatty": sys.stdin.isatty(), "pid": os.getpid(),
}
if name in ("SessionStart",) or not glob.glob(os.path.join(LOG, "parents_*.txt")):
    p = parents(); rec["parent_chain"] = p
    open(os.path.join(LOG, f"parents_{int(t0*1000)}.txt"), "w").write("\n".join(p))

cmd = ((ev.get("tool_input") or {}).get("command") or "") if isinstance(ev, dict) else ""
mode = None
if name == "PreToolUse" and "CASE_" in cmd:
    mode = cmd.split("CASE_", 1)[1].split()[0].strip("'\";&|")
rec["mode"] = mode

def flush(extra=None):
    rec["elapsed_ms"] = (time.time() - t0) * 1000
    if extra: rec["note"] = extra
    open(os.path.join(LOG, f"{int(t0*1000)}_{os.getpid()}_{name}.json"), "w", encoding="utf-8").write(json.dumps(rec, indent=1, default=str))

flush("start")
if mode == "exit1": sys.stderr.write("probe: exit 1\n"); sys.exit(1)
if mode == "exit2": sys.stderr.write("probe: blocked via exit 2 stderr\n"); sys.exit(2)
if mode == "blockjson": print(json.dumps({"decision": "block", "reason": "probe: blocked via JSON"})); sys.exit(0)
if mode == "approvejson": print(json.dumps({"decision": "approve", "reason": "probe approve"})); sys.exit(0)
if mode == "badjson": print("{not json"); sys.exit(0)
if mode == "garbage": print("hello plain text"); sys.exit(0)
if mode == "crash": os._exit(139)
if mode == "hang": time.sleep(60); sys.exit(0)
if mode == "slow": time.sleep(2); sys.exit(0)
if mode == "bigout": sys.stdout.write("x" * 2_000_000); sys.exit(0)
if mode == "exit2json": print(json.dumps({"decision": "approve"})); sys.exit(2)
flush("end")
sys.exit(0)
