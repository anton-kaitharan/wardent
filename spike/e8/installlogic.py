"""THROWAWAY E8 prototype: config-editing logic only. Not product code."""
import json, re, tomllib

MARK = "wardent hook"
EVENTS = {"UserPromptSubmit": None, "PreToolUse": "Bash|PowerShell|Edit|Write|apply_patch", "PostToolUse": "Bash|PowerShell|Edit|Write|apply_patch", "Stop": None}

class Refuse(Exception):
    pass

def _detect_style(raw: str):
    bom = raw.startswith("\ufeff")
    body = raw[1:] if bom else raw
    crlf = "\r\n" in body
    m = re.search(r"\n([ \t]+)\"", body.replace("\r\n", "\n"))
    indent = m.group(1) if m else "  "
    if indent.startswith("\t"): ind = "\t"
    else: ind = len(indent)
    return dict(bom=bom, crlf=crlf, indent=ind, final_nl=body.endswith("\n"), compact=("\n" not in body.strip()))

def _dump(obj, st):
    s = json.dumps(obj, indent=None if st["compact"] else st["indent"], ensure_ascii=False, separators=(",", ":") if st["compact"] else None)
    if st["crlf"]: s = s.replace("\n", "\r\n")
    if st["final_nl"]: s += "\r\n" if st["crlf"] else "\n"
    return ("\ufeff" if st["bom"] else "") + s

_OURS = re.compile(r"wardent\S*\s+hook\s+(claude|codex)\s+[a-z-]+")

def _our(entry, agent):
    return any(_OURS.search(h.get("command", "") or "") for h in entry.get("hooks", []))

def install_json(raw: str | None, agent: str, exe="wardent"):
    """Merge Wardent hooks into a Claude settings.json / Codex hooks.json text. Returns new text. Idempotent."""
    if raw is None or raw.strip() == "":
        doc, st = {}, dict(bom=False, crlf=False, indent=2, final_nl=True, compact=False)
    else:
        st = _detect_style(raw)
        try: doc = json.loads(raw.lstrip("\ufeff"))
        except Exception as e: raise Refuse(f"invalid JSON (comments/trailing commas?): {e}")
        if not isinstance(doc, dict): raise Refuse("top-level is not an object")
    hooks = doc.setdefault("hooks", {})
    if not isinstance(hooks, dict): raise Refuse("'hooks' is not an object")
    for ev, matcher in EVENTS.items():
        lst = hooks.setdefault(ev, [])
        if not isinstance(lst, list): raise Refuse(f"hooks.{ev} is not a list")
        lst[:] = [e for e in lst if not _our(e, agent)]       # drop stale/old versions of ours, keep everything else
        h = {"type": "command", "command": f"{exe} hook {agent} {re.sub(r'(?<!^)(?=[A-Z])', '-', ev).lower()}", "timeout": 5}
        entry = {"hooks": [h]}
        if matcher: entry = {"matcher": matcher, **entry}
        lst.append(entry)
    return _dump(doc, st)

def uninstall_json(raw: str, agent: str):
    st = _detect_style(raw)
    try: doc = json.loads(raw.lstrip("\ufeff"))
    except Exception as e: raise Refuse(str(e))
    hooks = doc.get("hooks", {})
    for ev in list(hooks):
        if isinstance(hooks[ev], list):
            hooks[ev] = [e for e in hooks[ev] if not _our(e, agent)]
            if not hooks[ev]: del hooks[ev]
    if not hooks: doc.pop("hooks", None)
    return _dump(doc, st)

# --- Codex config.toml: never parse-and-rewrite; managed block appended as text and validated with a real TOML parser
B0, B1 = "# >>> wardent managed (do not edit) >>>", "# <<< wardent managed <<<"

def install_toml(raw: str | None, exe="wardent"):
    raw = raw or ""
    raw = remove_block(raw)
    try: before = tomllib.loads(raw)
    except Exception as e: raise Refuse(f"existing TOML invalid: {e}")
    lines = [B0]
    for ev, matcher in EVENTS.items():
        lines.append(f"[[hooks.{ev}]]")
        if matcher: lines.append(f'matcher = "{matcher}"')
        lines.append(f"[[hooks.{ev}.hooks]]")
        lines += ['type = "command"', f'command = "{exe} hook codex {re.sub(r"(?<!^)(?=[A-Z])", "-", ev).lower()}"', "timeout = 5"]
    lines.append(B1)
    sep = "" if raw.endswith("\n") or raw == "" else "\n"
    out = raw + sep + ("\n" if raw else "") + "\n".join(lines) + "\n"
    try: after = tomllib.loads(out)
    except Exception as e: raise Refuse(f"appending block would make TOML invalid (e.g. 'hooks' already defined as inline table): {e}")
    for k in before:
        if k != "hooks" and before[k] != after[k]: raise Refuse("block changed unrelated keys")
    return out

def remove_block(raw: str):
    pat = re.compile(r"\n?" + re.escape(B0) + r".*?" + re.escape(B1) + r"\n?", re.S)
    return pat.sub("", raw)
