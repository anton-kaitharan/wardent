const fs = require('fs');
const [rulesP, polP, logP] = process.argv.slice(2);
const input = fs.readFileSync(0, 'utf8');
const ev = JSON.parse(input);
const rules = JSON.parse(fs.readFileSync(rulesP, 'utf8'));
const pol = JSON.parse(fs.readFileSync(polP, 'utf8'));
const SPECIAL = '.+^${}()|[]' + String.fromCharCode(92);
const g2r = g => {
  let b = '^';
  for (let i = 0; i < g.length; i++) {
    const c = g[i];
    if (c === '*' && g[i + 1] === '*') { if (g[i + 2] === '/') { b += '(.*/)?'; i += 2; } else { b += '.*'; i++; } }
    else if (c === '*') b += '[^/]*';
    else if (c === '?') b += '[^/]';
    else if (SPECIAL.includes(c)) b += String.fromCharCode(92) + c;
    else b += c;
  }
  return new RegExp(b + '$');
};
const res = rules.regexes.map(r => new RegExp(r, 'i'));
const gls = rules.globs.map(g2r);
const sec = rules.secrets.map(r => new RegExp(r));
const prs = pol.rules.map(r => new RegExp(r.match.command_regex, 'i'));
const ti = ev.tool_input || {};
const hits = [];
if (ti.command) { res.forEach((r, i) => { if (r.test(ti.command)) hits.push('re' + i); }); prs.forEach((r, i) => { if (r.test(ti.command)) hits.push('pol' + i); }); }
if (ti.file_path) { const p = ti.file_path.split(String.fromCharCode(92)).join('/'); gls.forEach((g, i) => { if (g.test(p)) hits.push('gl' + i); }); }
if (ti.content) sec.forEach((r, i) => { if (r.test(ti.content)) hits.push('sec' + i); });
fs.appendFileSync(logP, JSON.stringify({ts: Date.now(), ev: ev.hook_event_name, tool: ev.tool_name, n: input.length, hits}) + '\n');
if (hits.length) process.stdout.write(JSON.stringify({hookSpecificOutput: {hookEventName: 'PreToolUse', permissionDecision: 'ask', permissionDecisionReason: hits.join(',')}}));
