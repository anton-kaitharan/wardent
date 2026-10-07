import subprocess, shutil, time, os, json, sys, statistics
os.makedirs("bin/fresh", exist_ok=True)
res = {}
for name, src in [("go", "bin/shim-go.exe"), ("bun-compiled", "bin/shim-bun.exe")]:
    first, second, third = [], [], []
    for i in range(12):
        dst = f"bin/fresh/{name}_{i}_{int(time.time()*1000)}.exe"
        shutil.copyfile(src, dst)
        ts = []
        for k in range(3):
            with open("data/payload_1k.json", "rb") as f:
                t = time.perf_counter()
                subprocess.run([dst, "data/rules.json", "data/policy.json", "data/bench.log"], stdin=f, stdout=subprocess.DEVNULL)
                ts.append((time.perf_counter() - t) * 1000)
        first.append(ts[0]); second.append(ts[1]); third.append(ts[2])
        os.remove(dst)
    res[name] = dict(first=first, second=second, third=third)
    print(f"{name:14} first-run ms: median {statistics.median(first):7.1f} max {max(first):7.1f} | 2nd median {statistics.median(second):6.1f} | 3rd median {statistics.median(third):6.1f}")
json.dump(res, open("../results/E5/win_cold_fresh_binary.json", "w"), indent=1)
