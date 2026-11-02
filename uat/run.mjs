// CRAB browser and accessibility harness (R-17, R-18; T-245, T-246).
//
// Test-only (U-19). It starts the seeded UAT server from the Go end-to-end
// harness (TestUATServe), drives real browsers through journeys and runs axe
// on every screen in light and dark, then compares the results with
// known-defects.json:
//   - a failure listed there is a recorded defect (register ID) and passes;
//   - a failure not listed fails the run (a new defect);
//   - a listed case that now passes fails the run, so the list is kept true.
//
// Environment:
//   CRAB_TEST_POSTGRES_ADMIN_URL  needed to start the UAT server
//   UAT_BASE / UAT_CONTROL        use an already-running UAT server instead
//   UAT_BROWSERS                  default chromium,firefox,webkit
//   UAT_REQUIRE_ALL=1             fail if a browser or axe is unavailable (CI)
import { spawn } from 'node:child_process';
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { journeys } from './journeys.mjs';
import { screens, runAxe } from './axe.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const resultsDir = path.join(here, 'results');
const requireAll = process.env.UAT_REQUIRE_ALL === '1';
const require = createRequire(import.meta.url);

function load(name) {
  try {
    return require(name);
  } catch {
    // Local fallback: a globally installed copy (development containers).
    try {
      return require(require.resolve(name, { paths: ['/opt/npm-tools/node_modules', '/usr/lib/node_modules', '/usr/local/lib/node_modules'] }));
    } catch {
      return null;
    }
  }
}

function axePath() {
  for (const paths of [[here], ['/opt/npm-tools/node_modules', '/usr/lib/node_modules']]) {
    try {
      return require.resolve('axe-core/axe.min.js', { paths });
    } catch { /* try next */ }
  }
  return null;
}

async function startServer() {
  if (process.env.UAT_BASE && process.env.UAT_CONTROL) {
    return { info: { base: process.env.UAT_BASE, control: process.env.UAT_CONTROL, users: JSON.parse(process.env.UAT_USERS || '{}'), incident: 'INC-014', stress: 'INC-STRESS' }, stop: async () => {} };
  }
  const file = path.join(here, '.server.json');
  fs.rmSync(file, { force: true });
  const child = spawn('go', ['test', '-count=1', '-timeout', '60m', '-run', '^TestUATServe$', './cmd/crab'], {
    cwd: root, env: { ...process.env, CRAB_UAT_SERVE_FILE: file, CRAB_UAT_SERVE_SECONDS: '3000' }, stdio: ['ignore', 'pipe', 'pipe'],
  });
  let log = '';
  child.stdout.on('data', (d) => { log += d; });
  child.stderr.on('data', (d) => { log += d; });
  const deadline = Date.now() + 15 * 60 * 1000;
  while (!fs.existsSync(file)) {
    if (child.exitCode !== null) throw new Error('UAT server exited before it was ready:\n' + log);
    if (Date.now() > deadline) throw new Error('UAT server not ready in time:\n' + log);
    await new Promise((r) => setTimeout(r, 500));
  }
  const info = JSON.parse(fs.readFileSync(file, 'utf8'));
  return {
    info,
    stop: async () => {
      await fetch(info.control + '/stop', { method: 'POST' }).catch(() => {});
      await new Promise((r) => { if (child.exitCode !== null) r(); else child.on('exit', r); });
    },
  };
}

async function main() {
  const pw = load('playwright');
  if (!pw) throw new Error('playwright is not installed: run npm ci in uat/');
  const axe = axePath();
  const wanted = (process.env.UAT_BROWSERS || 'chromium,firefox,webkit').split(',').map((s) => s.trim()).filter(Boolean);
  const known = JSON.parse(fs.readFileSync(path.join(here, 'known-defects.json'), 'utf8'));
  fs.mkdirSync(resultsDir, { recursive: true });
  const results = [];
  const record = (id, ok, detail = '') => {
    results.push({ id, ok, detail });
    process.stdout.write(`${ok ? 'ok  ' : 'FAIL'} ${id}${detail ? ' — ' + detail : ''}\n`);
  };
  const unavailable = [];
  const server = await startServer();
  try {
    for (const name of wanted) {
      let browser;
      try {
        browser = await pw[name].launch();
      } catch (err) {
        unavailable.push(`${name}: ${String(err.message).split('\n')[0]}`);
        continue;
      }
      try {
        const env = { pw, browser, name, info: server.info, resultsDir };
        for (const j of journeys) {
          if (j.only && !j.only.includes(name)) continue;
          const id = `journey:${j.id}:${name}`;
          try {
            const detail = await j.run(env);
            record(id, true, detail || '');
          } catch (err) {
            record(id, false, String(err.message).split('\n')[0]);
          }
        }
        if (axe) {
          for (const scheme of ['light', 'dark']) {
            for (const s of screens) {
              const id = `axe:${s.id}:${scheme}:${name}`;
              try {
                const found = await runAxe(env, axe, s, scheme);
                record(id, found.length === 0, found.join('; '));
              } catch (err) {
                record(id, false, 'could not run: ' + String(err.message).split('\n')[0]);
              }
            }
          }
        } else {
          unavailable.push('axe-core: not installed');
        }
      } finally {
        await browser.close();
      }
    }
  } finally {
    await server.stop();
  }

  // Compare with the recorded defects.
  const seen = new Set(results.map((r) => r.id));
  const report = { generatedAt: new Date().toISOString(), unavailable, cases: [] };
  let unexpected = 0;
  for (const r of results) {
    const defect = known[r.id];
    let verdict;
    if (r.ok && defect) { verdict = 'UNEXPECTED PASS — remove from known-defects.json'; unexpected++; }
    else if (r.ok) verdict = 'pass';
    else if (defect) verdict = `known defect ${defect}`;
    else { verdict = 'NEW DEFECT — record it in the register and known-defects.json'; unexpected++; }
    report.cases.push({ ...r, verdict });
  }
  const axeRan = !unavailable.some((u) => u.startsWith('axe-core'));
  const browserRan = (id) => wanted.some((b) => id.endsWith(':' + b) && !unavailable.some((u) => u.startsWith(b + ':')));
  const stale = Object.keys(known).filter((id) => !seen.has(id) && browserRan(id) && (axeRan || !id.startsWith('axe:')));
  for (const id of stale) { report.cases.push({ id, ok: null, detail: '', verdict: 'NOT RUN — listed defect has no case' }); unexpected++; }
  if (requireAll && unavailable.length) unexpected++;
  fs.writeFileSync(path.join(resultsDir, 'report.json'), JSON.stringify(report, null, 2));
  const md = ['# CRAB browser and accessibility run', '', `Generated ${report.generatedAt}.`, '',
    unavailable.length ? `Unavailable: ${unavailable.join('; ')}` : 'All requested browsers and axe ran.', '',
    '| Case | Result | Detail |', '|---|---|---|',
    ...report.cases.map((c) => `| ${c.id} | ${c.verdict} | ${(c.detail || '').replace(/\|/g, '\\|').slice(0, 300)} |`)].join('\n');
  fs.writeFileSync(path.join(resultsDir, 'report.md'), md + '\n');
  const failed = report.cases.filter((c) => c.ok === false).length;
  process.stdout.write(`\n${report.cases.length} cases: ${failed} failing (${failed - report.cases.filter((c) => c.verdict === 'NEW DEFECT — record it in the register and known-defects.json').length} known), ${unexpected} unexpected.\n`);
  if (unavailable.length) process.stdout.write(`Unavailable: ${unavailable.join('; ')}\n`);
  process.exit(unexpected ? 1 : 0);
}

main().catch((err) => { console.error(err); process.exit(2); });
