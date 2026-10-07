import json, unittest, tomllib
from installlogic import *

USER_HOOK = {"hooks": [{"type": "command", "command": "echo user-hook"}]}
def mk(obj, indent=2, crlf=False, bom=False, nl=True, compact=False):
    s = json.dumps(obj, indent=None if compact else indent, separators=(",", ":") if compact else None)
    if crlf: s = s.replace("\n", "\r\n")
    if nl: s += "\r\n" if crlf else "\n"
    return ("\ufeff" if bom else "") + s

BASE = {"model": "opus", "permissions": {"allow": ["Bash(npm test)"], "deny": ["Read(.env)"]}, "hooks": {"PreToolUse": [{"matcher": "Bash", **USER_HOOK}]}}

class T(unittest.TestCase):
    def test_roundtrip_identity_per_style(self):
        for kw in [dict(), dict(indent=4), dict(indent="\t"), dict(crlf=True), dict(bom=True), dict(nl=False), dict(compact=True), dict(crlf=True, bom=True, indent=4)]:
            raw = mk(BASE, **kw)
            self.assertEqual(uninstall_json(install_json(raw, "claude"), "claude"), raw, kw)

    def test_idempotent(self):
        raw = mk(BASE); once = install_json(raw, "claude")
        self.assertEqual(install_json(once, "claude"), once)

    def test_preserves_user_hooks_and_other_keys(self):
        d = json.loads(install_json(mk(BASE), "claude"))
        self.assertEqual(d["model"], "opus"); self.assertEqual(d["permissions"], BASE["permissions"])
        cmds = [h["command"] for e in d["hooks"]["PreToolUse"] for h in e["hooks"]]
        self.assertIn("echo user-hook", cmds); self.assertEqual(sum("wardent hook" in c for c in cmds), 1)

    def test_missing_and_empty_file(self):
        for raw in (None, "", "  \n"):
            out = install_json(raw, "claude"); self.assertIn("wardent hook claude pre-tool-use", out)
            self.assertEqual(uninstall_json(out, "claude").strip(), "{}")

    def test_refuses_invalid_or_jsonc(self):
        for bad in ('{"a": 1,}', '{ // c\n "a": 1 }', "[1,2]", '{"hooks": []}', '{"hooks": {"PreToolUse": {}}}', "not json"):
            with self.assertRaises(Refuse, msg=bad): install_json(bad, "claude")

    def test_upgrade_replaces_old_entry(self):
        old = install_json(mk(BASE), "claude", exe="wardent-old")
        new = install_json(old, "claude")
        self.assertNotIn("wardent-old", new); self.assertEqual(new.count("wardent hook claude pre-tool-use"), 1)

    def test_key_order_and_unicode_preserved(self):
        raw = '{\n  "z": 1,\n  "a": "caf\u00e9",\n  "hooks": {}\n}\n'
        out = install_json(raw, "claude")
        self.assertTrue(out.index('"z"') < out.index('"a"')); self.assertIn("caf\u00e9", out)
        # KNOWN LIMITATION (finding): a pre-existing empty "hooks": {} is dropped on uninstall; byte-identical restore needs a backup+manifest
        self.assertEqual(uninstall_json(out, "claude").count('"hooks"'), 0)

    def test_toml_block_roundtrip_and_validity(self):
        raw = 'model = "gpt-5"\n[features]\nhooks = true\n'
        out = install_toml(raw); d = tomllib.loads(out)
        self.assertEqual(len(d["hooks"]["PreToolUse"]), 1); self.assertEqual(d["features"], {"hooks": True})
        self.assertEqual(remove_block(out), raw)
        self.assertEqual(install_toml(out), out)          # idempotent

    def test_toml_coexists_with_user_hook_tables(self):
        raw = '[[hooks.PreToolUse]]\nmatcher = "Bash"\n[[hooks.PreToolUse.hooks]]\ntype = "command"\ncommand = "echo mine"\n'
        d = tomllib.loads(install_toml(raw)); self.assertEqual(len(d["hooks"]["PreToolUse"]), 2)

    def test_toml_conflict_with_inline_hooks_is_refused(self):
        raw = 'hooks = { PreToolUse = [] }\n'
        with self.assertRaises(Refuse): install_toml(raw)

    def test_toml_invalid_input_refused(self):
        with self.assertRaises(Refuse): install_toml("[broken\n")

    def test_toml_no_trailing_newline(self):
        out = install_toml('model = "x"'); tomllib.loads(out); self.assertEqual(remove_block(out).strip(), 'model = "x"')

if __name__ == "__main__": unittest.main(verbosity=2)
