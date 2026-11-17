// npm audit for production deps that fails on any high/critical advisory,
// except ones in audit-allowlist.json whose "until" date hasn't passed.
// npm has no built-in ignore list, hence this script.
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

const BLOCKING = new Set(['high', 'critical']);
const allowlist = JSON.parse(readFileSync(new URL('../audit-allowlist.json', import.meta.url), 'utf8'));
const today = new Date().toISOString().slice(0, 10);

let raw;
try {
  raw = execFileSync('npm', ['audit', '--omit=dev', '--json'], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'inherit'] });
} catch (e) {
  raw = e.stdout; // npm audit exits non-zero whenever it finds anything
}
const report = JSON.parse(raw);

// Each vulnerable package lists the advisories behind it; collect the unique ones.
const advisories = new Map();
for (const vuln of Object.values(report.vulnerabilities ?? {})) {
  for (const via of vuln.via) {
    if (typeof via === 'object' && BLOCKING.has(via.severity)) {
      advisories.set(via.url.split('/').pop(), via);
    }
  }
}

let failed = false;
for (const [id, adv] of advisories) {
  const allowed = allowlist[id];
  if (allowed && today <= allowed.until) {
    console.log(`allowed until ${allowed.until}: ${id} ${adv.name} — ${adv.title}`);
  } else {
    failed = true;
    const why = allowed ? ` (allowlist entry expired ${allowed.until})` : '';
    console.error(`${adv.severity.toUpperCase()}: ${id} ${adv.name} ${adv.range} — ${adv.title}${why}\n  ${adv.url}`);
  }
}

for (const [id, { until }] of Object.entries(allowlist)) {
  if (!advisories.has(id)) console.log(`note: ${id} no longer reported — remove it from audit-allowlist.json (was allowed until ${until})`);
}

if (failed) process.exit(1);
console.log('npm audit: no unallowed high/critical advisories in production deps');
