const fs = require('fs');
const sq = require('shell-quote');
const [inP, mode] = process.argv.slice(2);
const items = JSON.parse(fs.readFileSync(inP, 'utf8'));
const wrappers = new Set(['sudo', 'doas', 'env', 'nohup', 'time', 'command', 'exec', 'timeout', 'nice', 'xargs', 'builtin']);
const flagArg = { sudo: ['-u', '-g', '-h', '-p', '-C'], timeout: ['-s', '-k'], nice: ['-n'], xargs: ['-I', '-n', '-P', '-L', '-s', '-d'], env: ['-u', '-C'] };
const isAssign = s => { const i = s.indexOf('='); return i > 0 && !/[\/ -]/.test(s.slice(0, i)); };

function handle(args, depth, out, extract) {
  let i = 0;
  while (i < args.length && isAssign(args[i])) i++;
  if (i >= args.length) return;
  const name = args[i], rest = args.slice(i + 1);
  if (wrappers.has(name)) {
    let j = 0;
    while (j < rest.length) {
      const a = rest[j];
      if (name === 'timeout' && /^[0-9]/.test(a)) { j++; continue; }
      if (a.startsWith('-') && a !== '-') { j += (flagArg[name] || []).includes(a) ? 2 : 1; continue; }
      if (isAssign(a) && (name === 'env' || name === 'sudo')) { j++; continue; }
      break;
    }
    if (j < rest.length) handle(rest.slice(j), depth, out, extract);
    return;
  }
  if (['bash', 'sh', 'zsh', 'dash'].includes(name)) {
    const k = rest.indexOf('-c');
    if (k >= 0 && k + 1 < rest.length) { if (rest[k + 1] === '<dyn>') out.push('<dyn>'); else extract(rest[k + 1], depth + 1, out); return; }
  }
  if (name === 'find') {
    out.push('find');
    rest.forEach((a, k) => { if ((a === '-exec' || a === '-execdir') && k + 1 < rest.length) handle(rest.slice(k + 1).filter(x => x !== ';'), depth, out, extract); });
    return;
  }
  out.push(name);
}

// ---- shell-quote tokenizer variant
function extractSQ(src, depth, out) {
  if (depth > 4) return true;
  let toks;
  try { toks = sq.parse(src, () => '$__V__'); } catch (e) { return false; }
  let cur = [];
  const flush = () => { if (cur.length) handle(cur, depth, out, extractSQ); cur = []; };
  for (const t of toks) {
    if (typeof t === 'string') cur.push(t.includes('$__V__') ? '<dyn>' : t);
    else if (t && t.op) { if (['|', '&&', '||', ';', '&', '|&'].includes(t.op)) flush(); }
    else if (t && t.comment) { /* ignore */ }
  }
  flush();
  return true;
}

async function main() {
  const res = [];
  if (mode === 'shell-quote') {
    for (const it of items) { const t = process.hrtime.bigint(); const out = []; const ok = extractSQ(it.cmd, 0, out); res.push({ id: it.id, ok, exes: out, micros: Number(process.hrtime.bigint() - t) / 1000 }); }
  } else if (mode === 'tree-sitter') {
    const { Parser, Language } = require('web-tree-sitter');
    await Parser.init();
    const lang = await Language.load(require.resolve('tree-sitter-wasms/out/tree-sitter-bash.wasm'));
    const parser = new Parser(); parser.setLanguage(lang);
    const wordOf = n => {
      const hasDyn = (x) => ['simple_expansion', 'expansion', 'command_substitution', 'arithmetic_expansion', 'process_substitution'].includes(x.type) || x.namedChildren.some(hasDyn);
      if (hasDyn(n)) return '<dyn>';
      let s = n.text;
      if (n.type === 'raw_string') s = s.slice(1, -1);
      else if (n.type === 'string') s = s.slice(1, -1);
      else if (n.type === 'concatenation') s = n.namedChildren.map(wordOf).join('');
      else s = s.replace(/\\(.)/g, '$1');
      return s;
    };
    const ex = (src, depth, out) => {
      if (depth > 4) return true;
      const tree = parser.parse(src);
      const ok = !tree.rootNode.hasError;
      const walk = n => {
        if (n.type === 'command') {
          const args = [];
          for (const c of n.namedChildren) {
            if (c.type === 'variable_assignment') { if (!args.length) continue; }
            else if (c.type === 'command_name') args.push(wordOf(c.firstNamedChild || c));
            else if (['file_redirect', 'heredoc_redirect', 'herestring_redirect'].includes(c.type)) continue;
            else args.push(wordOf(c));
          }
          if (args.length) handle(args, depth, out, ex);
        }
        for (const c of n.namedChildren) walk(c);
      };
      walk(tree.rootNode);
      return ok;
    };
    for (const it of items) { const t = process.hrtime.bigint(); const out = []; const ok = ex(it.cmd, 0, out); res.push({ id: it.id, ok, exes: out, micros: Number(process.hrtime.bigint() - t) / 1000 }); }
  }
  console.log(JSON.stringify(res));
}
main().catch(e => { console.error('FAIL', e.message); process.exit(3); });
