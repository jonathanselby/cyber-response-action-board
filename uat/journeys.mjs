// T-245 journeys. Each run(env) throws on failure and may return a detail.
import http from 'node:http';
import path from 'node:path';

export async function count(info, what) {
  const res = await fetch(`${info.control}/count/${what}`);
  return (await res.json()).count;
}

export async function signIn(page, info, user = 'alice') {
  await page.goto(info.base + '/');
  await page.fill('input[name="human_id"]', user);
  await page.fill('input[name="password"]', info.users[user]);
  await Promise.all([page.waitForLoadState('load'), page.click('form[action="/login"] button')]);
  if (await page.locator('form[action="/login"]').count()) throw new Error(`${user} could not sign in`);
}

export async function openAll(page) {
  await page.evaluate(() => document.querySelectorAll('details').forEach((d) => { d.open = true; }));
}

// fillForm fills the first form whose action ends with suffix and submits it.
async function submitForm(page, suffix, values) {
  await openAll(page);
  const form = page.locator(`form[action$="${suffix}"]`).first();
  for (const [name, value] of Object.entries(values)) await form.locator(`[name="${name}"]`).fill(value);
  await Promise.all([page.waitForLoadState('load'), form.locator('button').last().click()]);
}

async function newPage(env, options = {}) {
  const context = await env.browser.newContext(options);
  return { context, page: await context.newPage() };
}

function unique(prefix) {
  return `${prefix} ${Date.now().toString(36)}${Math.floor(Math.random() * 1e6).toString(36)}`;
}

const board = (info) => `${info.base}/incident/${info.incident}`;

// Screens checked for reflow and phone layout.
export const layoutScreens = (info) => ['/', '/approvals', '/intake', '/readiness', '/organisation', '/access',
  `/incident/${info.incident}`, `/incident/${info.incident}/evidence`, `/incident/${info.incident}/communications`,
  `/incident/${info.incident}/access`, `/incident/${info.incident}/close`, `/incident/${info.stress}`];

async function overflowing(page) {
  return page.evaluate(() => {
    const doc = document.documentElement;
    return doc.scrollWidth > window.innerWidth + 1 ? `${doc.scrollWidth}px wide in a ${window.innerWidth}px viewport` : '';
  });
}

async function reflow(env, width) {
  const { context, page } = await newPage(env, { viewport: { width, height: 900 } });
  try {
    await signIn(page, env.info);
    const bad = [];
    for (const p of layoutScreens(env.info)) {
      await page.goto(env.info.base + p);
      const o = await overflowing(page);
      if (o) bad.push(`${p} ${o}`);
    }
    if (bad.length) throw new Error(`horizontal scrolling at ${width}px: ${bad.join('; ')}`);
  } finally {
    await context.close();
  }
}

export const journeys = [
  {
    id: 'refresh-and-back-never-resubmit',
    async run(env) {
      const { context, page } = await newPage(env);
      try {
        await signIn(page, env.info);
        await page.goto(board(env.info));
        const before = await count(env.info, 'facts');
        await submitForm(page, '/fact', { statement: unique('PRG fact'), source: 'harness' });
        if (await count(env.info, 'facts') !== before + 1) throw new Error('the fact was not recorded once');
        await page.reload();
        await page.goBack();
        await page.goForward();
        await page.reload();
        const after = await count(env.info, 'facts');
        if (after !== before + 1) throw new Error(`refresh or back resubmitted: ${after - before} facts recorded`);
      } finally {
        await context.close();
      }
    },
  },
  {
    id: 'multiple-tabs',
    async run(env) {
      const { context, page: a } = await newPage(env);
      try {
        await signIn(a, env.info);
        const b = await context.newPage();
        await a.goto(board(env.info));
        await b.goto(board(env.info));
        const text = unique('Two-tab fact');
        await submitForm(a, '/fact', { statement: text, source: 'tab A' });
        await b.reload();
        if (!(await b.content()).includes(text)) throw new Error('the second tab does not show what the first recorded');
        // Signing out in one tab ends the session in the other.
        await a.evaluate(async () => { await fetch('/logout', { method: 'POST' }); });
        await b.reload();
        if (!(await b.locator('form[action="/login"]').count())) throw new Error('the second tab still works after signing out in the first');
      } finally {
        await context.close();
      }
    },
  },
  {
    id: 'visible-sign-out',
    async run(env) {
      const { context, page } = await newPage(env);
      try {
        await signIn(page, env.info);
        const n = await page.locator('form[action="/logout"]').count();
        if (!n) throw new Error('no sign-out control on the Command Summary');
      } finally {
        await context.close();
      }
    },
  },
  {
    id: 'expired-session-keeps-unfinished-form',
    async run(env) {
      const { context, page } = await newPage(env);
      try {
        await signIn(page, env.info);
        await page.goto(board(env.info));
        const text = unique('Typed before expiry');
        await openAll(page);
        const form = page.locator('form[action$="/fact"]').first();
        await form.locator('[name="statement"]').fill(text);
        await form.locator('[name="source"]').fill('harness');
        await fetch(`${env.info.control}/expire/alice`, { method: 'POST' });
        await Promise.all([page.waitForLoadState('load'), form.locator('button').last().click()]);
        const body = await page.content();
        if (!body.includes(text) && !/not (been )?(saved|recorded)/i.test(body)) {
          throw new Error('after the session expired the typed text was lost without saying so');
        }
      } finally {
        await context.close();
      }
    },
  },
  {
    id: 'interrupted-upload-leaves-no-evidence',
    only: ['chromium'], // server behaviour; one browser is enough
    async run(env) {
      const { context, page } = await newPage(env);
      try {
        await signIn(page, env.info);
        const cookie = (await context.cookies()).map((c) => `${c.name}=${c.value}`).join('; ');
        const before = { evidence: await count(env.info, 'evidence'), blobs: await count(env.info, 'blobs') };
        const url = new URL(`${board(env.info)}/evidence`);
        const boundary = 'crabuat' + Date.now();
        const head = [`--${boundary}`, 'Content-Disposition: form-data; name="collector"', '', 'alice',
          `--${boundary}`, 'Content-Disposition: form-data; name="source"', '', 'laptop',
          `--${boundary}`, 'Content-Disposition: form-data; name="current_handler"', '', 'alice',
          `--${boundary}`, 'Content-Disposition: form-data; name="evidence"; filename="memory.raw"', 'Content-Type: application/octet-stream', '', ''].join('\r\n');
        await new Promise((resolve) => {
          const req = http.request({ host: url.hostname, port: url.port, path: url.pathname + '?compartment=GENERAL', method: 'POST',
            headers: { Cookie: cookie, Origin: env.info.base, 'Content-Type': `multipart/form-data; boundary=${boundary}`, 'Content-Length': head.length + 8 * 1024 * 1024 } });
          req.on('error', () => resolve());
          req.write(head);
          req.write(Buffer.alloc(512 * 1024, 7));
          setTimeout(() => { req.destroy(); resolve(); }, 300);
        });
        await new Promise((r) => setTimeout(r, 1500));
        const after = { evidence: await count(env.info, 'evidence'), blobs: await count(env.info, 'blobs') };
        if (after.evidence !== before.evidence) throw new Error('an interrupted upload created an evidence record');
        if (after.blobs !== before.blobs) throw new Error(`an interrupted upload left ${after.blobs - before.blobs} stored blob(s) with no evidence record`);
      } finally {
        await context.close();
      }
    },
  },
  { id: 'reflow-200-percent', run: (env) => reflow(env, 640) },
  { id: 'reflow-400-percent', run: (env) => reflow(env, 320) },
  {
    id: 'keyboard-only',
    async run(env) {
      const { context, page } = await newPage(env);
      try {
        await page.goto(env.info.base + '/');
        const focusName = () => page.evaluate(() => document.activeElement && (document.activeElement.getAttribute('name') || document.activeElement.textContent.trim().slice(0, 40)));
        let reached = false;
        for (let i = 0; i < 15 && !reached; i++) {
          await page.keyboard.press('Tab');
          reached = (await focusName()) === 'human_id';
        }
        if (!reached) throw new Error('the sign-in field cannot be reached with Tab');
        await page.keyboard.type('alice');
        await page.keyboard.press('Tab');
        if ((await focusName()) !== 'password') throw new Error('Tab from the ID does not reach the password');
        await page.keyboard.type(env.info.users.alice);
        await Promise.all([page.waitForLoadState('load'), page.keyboard.press('Enter')]);
        if (await page.locator('form[action="/login"]').count()) throw new Error('Enter did not sign in');
        // Every focus stop on the Command Summary shows where focus is.
        await page.keyboard.press('Tab');
        const first = await focusName();
        if (!/skip/i.test(first || '')) throw new Error(`the first Tab stop is "${first}", not a skip link`);
        const invisible = [];
        for (let i = 0; i < 25; i++) {
          const v = await page.evaluate(() => {
            const el = document.activeElement;
            if (!el || el === document.body) return null;
            const s = getComputedStyle(el);
            const visible = (s.outlineStyle !== 'none' && parseFloat(s.outlineWidth) > 0) || (s.boxShadow && s.boxShadow !== 'none');
            return visible ? '' : (el.tagName.toLowerCase() + ':' + (el.textContent || el.getAttribute('name') || '').trim().slice(0, 30));
          });
          if (v) invisible.push(v);
          await page.keyboard.press('Tab');
        }
        if (invisible.length) throw new Error(`focus not visible on: ${[...new Set(invisible)].join(', ')}`);
      } finally {
        await context.close();
      }
    },
  },
  {
    id: 'phone',
    only: ['chromium', 'webkit'], // Firefox has no mobile emulation
    async run(env) {
      const device = env.pw.devices[env.name === 'webkit' ? 'iPhone 13' : 'Pixel 7'];
      const { context, page } = await newPage(env, { ...device });
      try {
        await signIn(page, env.info);
        const bad = [];
        for (const p of layoutScreens(env.info)) {
          await page.goto(env.info.base + p);
          const o = await overflowing(page);
          if (o) bad.push(`${p} ${o}`);
          const cut = await page.evaluate(() => [...document.querySelectorAll('header *')].filter((el) => {
            const s = getComputedStyle(el);
            return s.textOverflow === 'ellipsis' && el.scrollWidth > el.clientWidth;
          }).map((el) => el.textContent.trim().slice(0, 40)));
          if (cut.length) bad.push(`${p} header text cut off: ${cut.join(', ')}`);
        }
        await page.goto(env.info.base + `/incident/${env.info.stress}`);
        await page.screenshot({ path: path.join(env.resultsDir, `stress-board-${env.name}-phone.png`), fullPage: true });
        if (bad.length) throw new Error(bad.join('; '));
      } finally {
        await context.close();
      }
    },
  },
  {
    id: 'stress-board-capture',
    async run(env) {
      const { context, page } = await newPage(env, { viewport: { width: 1366, height: 900 } });
      try {
        await signIn(page, env.info);
        const start = Date.now();
        await page.goto(env.info.base + `/incident/${env.info.stress}`);
        const ms = Date.now() - start;
        await page.screenshot({ path: path.join(env.resultsDir, `stress-board-${env.name}-desktop.png`), fullPage: true });
        const height = await page.evaluate(() => document.documentElement.scrollHeight);
        return `loaded in ${ms} ms, page ${height}px tall`;
      } finally {
        await context.close();
      }
    },
  },
];
