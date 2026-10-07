import json, subprocess, sys, os, statistics, platform, time, glob, shutil
EXE = ".exe" if os.name == "nt" else ""
HF = (glob.glob("../tools/hyperfine/*/hyperfine.exe") or ["hyperfine"])[0]
cond = sys.argv[1]            # idle | busy
runs = int(sys.argv[2]) if len(sys.argv) > 2 else 200
out = f"../results/E5/{platform.system().lower()}_{cond}" + (sys.argv[3] if len(sys.argv) > 3 else "")
os.makedirs(out, exist_ok=True)
R, P, L = "data/rules.json", "data/policy.json", "data/bench.log"
cands = {
 "node-plain":        ["node", "shim.js"],
 "node-bundled":      ["node", "dist/shim.bundle.js"],
 "node-bundled-cc":   ["node", "shim_cc.js"],
 "bun-script":        ["bun", "shim.js"],
 "bun-compiled":      ["bin/shim-bun" + EXE],
 "go":                ["bin/shim-go" + EXE],
}
floors = {"floor-node-noop": ["node", "noop.js"], "floor-go-noop": ["bin/noop-go" + EXE, "noop"]}
burner = None
if cond == "busy":
    burner = subprocess.Popen([sys.executable, "-c", "while True: pass"])
    time.sleep(1)
def run(name, argv, payload):
    argv = ["./" + argv[0]] + argv[1:] if (os.name != "nt" and argv[0].startswith("bin/")) else argv
    cmd = " ".join(argv + ([R, P, L] if not name.startswith("floor") else []))
    j = f"{out}/{name}__{payload}.json"
    subprocess.run([HF, "-N", "--warmup", "10", "--runs", str(runs), "--input", f"data/{payload}.json", "--export-json", j, cmd], stdout=subprocess.DEVNULL, check=True)
    t = sorted(json.load(open(j))["results"][0]["times"])
    ms = lambda q: t[min(len(t)-1, int(q*len(t)))]*1000
    return dict(name=name, payload=payload, n=len(t), p50=ms(.5), p95=ms(.95), p99=ms(.99), max=t[-1]*1000, over200=sum(1 for x in t if x > .2))
rows = []
try:
    for name, argv in floors.items(): rows.append(run(name, argv, "payload_1k"))
    for payload in ["payload_1k", "payload_1m"]:
        for name, argv in cands.items(): rows.append(run(name, argv, payload))
finally:
    if burner: burner.kill()
json.dump(dict(env=dict(os=platform.platform(), cpu=platform.processor(), cond=cond, node=subprocess.getoutput("node --version"), bun=subprocess.getoutput("bun --version"), go=subprocess.getoutput("go version")), rows=rows), open(f"{out}/summary.json", "w"), indent=1)
print(f"{'candidate':18}{'payload':13}{'p50':>8}{'p95':>8}{'p99':>8}{'max':>8}{'>200ms':>7}")
for r in rows: print(f"{r['name']:18}{r['payload']:13}{r['p50']:8.1f}{r['p95']:8.1f}{r['p99']:8.1f}{r['max']:8.1f}{r['over200']:7d}")
