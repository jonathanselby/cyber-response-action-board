// T-246: axe on every screen, light and dark. Serious and critical
// violations fail; moderate and minor are not counted.
import { signIn } from './journeys.mjs';

const inc = (info, suffix = '') => `/incident/${info.incident}${suffix}`;

export const screens = [
  { id: 'sign-in', user: null, path: () => '/' },
  { id: 'enrol', user: null, path: () => '/enrol' },
  { id: 'command-summary', path: () => '/' },
  { id: 'approvals', path: () => '/approvals' },
  { id: 'first-reports', path: () => '/intake' },
  { id: 'readiness', path: () => '/readiness' },
  { id: 'organisation', path: () => '/organisation' },
  { id: 'obligation-library', path: () => '/obligations' },
  { id: 'access-admin', path: () => '/access' },
  { id: 'account', path: () => '/account' },
  { id: 'board', path: (info) => inc(info) },
  { id: 'stress-board', path: (info) => `/incident/${info.stress}` },
  { id: 'evidence', path: (info) => inc(info, '/evidence') },
  {
    id: 'evidence-item',
    async path(info, page) {
      await page.goto(info.base + inc(info, '/evidence'));
      const href = await page.locator('a[href^="/evidence/"]').first().getAttribute('href');
      if (!href) throw new Error('no evidence item to open');
      return href;
    },
  },
  { id: 'communications', path: (info) => inc(info, '/communications') },
  { id: 'incident-access', path: (info) => inc(info, '/access') },
  { id: 'close-out', path: (info) => inc(info, '/close') },
];

const impactsThatFail = new Set(['serious', 'critical']);

export async function runAxe(env, axePath, screen, scheme) {
  // bypassCSP lets the harness inject axe; CRAB pages load no scripts.
  const context = await env.browser.newContext({ colorScheme: scheme, bypassCSP: true, viewport: { width: 1280, height: 900 } });
  try {
    const page = await context.newPage();
    if (screen.user !== null) await signIn(page, env.info, screen.user || 'alice');
    const target = await screen.path(env.info, page);
    await page.goto(env.info.base + target);
    await page.evaluate(() => document.querySelectorAll('details').forEach((d) => { d.open = true; }));
    await page.addScriptTag({ path: axePath });
    const violations = await page.evaluate(async () => {
      // eslint-disable-next-line no-undef
      const r = await axe.run(document, { resultTypes: ['violations'] });
      return r.violations.map((v) => ({ id: v.id, impact: v.impact, nodes: v.nodes.length, target: v.nodes[0] && v.nodes[0].target.join(' ') }));
    });
    return violations.filter((v) => impactsThatFail.has(v.impact)).map((v) => `${v.id} (${v.impact}, ${v.nodes} node${v.nodes === 1 ? '' : 's'}, e.g. ${v.target})`);
  } finally {
    await context.close();
  }
}
