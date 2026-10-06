import { test, beforeAll, afterAll } from 'e2e';
import assert from 'node:assert/strict';
import { spawn, execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { mkdtemp, readFile, writeFile, chmod, stat, lstat, link, symlink, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import http from 'node:http';
import https from 'node:https';
import net from 'node:net';

const exec = promisify(execFile);
const email = 'parent@example.test';
const password = 'fixture-password';
const token = 'fixture-csrf';
const cookie = 'fixture-session';
let suiteDir: string;
let binary: string;
let certificate: Buffer;
let key: Buffer;
let buildCommit = 'unknown';
let buildDate = 'unknown';

beforeAll(async () => {
  suiteDir = await mkdtemp(join(tmpdir(), 'edcctl-e2e-'));
  binary = join(suiteDir, 'edcctl');
  await exec('go', ['build', '-trimpath', '-o', binary, './cmd/edcctl'], { timeout: 120_000 });
  const { stdout: buildInfo } = await exec('go', ['version', '-m', binary]);
  buildCommit = buildInfo.match(/vcs\.revision=(\S+)/)?.[1] ?? 'unknown';
  buildDate = buildInfo.match(/vcs\.time=(\S+)/)?.[1] ?? 'unknown';
  await exec('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
    '-subj', '/CN=mobileinventor.com', '-addext', 'subjectAltName=DNS:mobileinventor.com',
    '-keyout', join(suiteDir, 'key.pem'), '-out', join(suiteDir, 'cert.pem')]);
  certificate = await readFile(join(suiteDir, 'cert.pem'));
  key = await readFile(join(suiteDir, 'key.pem'));
});

afterAll(async () => { await rm(suiteDir, { recursive: true, force: true }); });

type Result = { code: number | null; stdout: string; stderr: string };
type Request = { method: string; url: URL; headers: http.IncomingHttpHeaders; form: URLSearchParams; app: boolean };
type Options = {
  login?: 'reject' | 'missing-token' | 'wrong-page' | 'redirect' | 'unauthorized' | 'slow';
  schedule?: unknown;
  payments?: string;
  history?: string;
  app?: Record<string, unknown>;
  status?: number;
};

async function listen(server: http.Server | https.Server) {
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  return (server.address() as net.AddressInfo).port;
}

async function fixture(options: Options, body: (f: {
  run: (args: string[], extra?: Record<string, string>, input?: string) => Promise<Result>;
  requests: Request[]; home: string; config: string; baseURL: string;
}) => Promise<void>) {
  const home = await mkdtemp(join(suiteDir, 'home-'));
  const config = join(home, 'config.yaml');
  const requests: Request[] = [];
  const failures: Error[] = [];
  const handler = (app: boolean) => async (req: http.IncomingMessage, res: http.ServerResponse) => {
    try {
      let data = '';
      for await (const chunk of req) data += chunk;
      const url = new URL(req.url!, 'https://fixture.test');
      const request = { method: req.method!, url, headers: req.headers, form: new URLSearchParams(data), app };
      requests.push(request);
      if (app) {
        assert.equal(req.headers.cookie, undefined, 'portal cookies cross app boundary');
        assert.equal(req.headers['x-csrf-token'], undefined, 'portal CSRF crosses app boundary');
        assert.ok(!data.includes(password), 'password crosses app boundary');
        assert.equal(url.searchParams.get('account_id'), null);
        if (options.status) { res.writeHead(options.status); res.end('fixture error'); return; }
        res.setHeader('content-type', 'application/json');
        res.end(JSON.stringify(options.app && Object.hasOwn(options.app, url.pathname) ? options.app[url.pathname] : appResponse(request)));
        return;
      }
      assert.equal(url.searchParams.get('app'), '1');
      assert.equal(url.searchParams.get('app_mi'), '1');
      if (url.pathname === '/online/index.php' && req.method === 'GET') {
        if (options.login === 'slow') { return; }
        if (options.login === 'unauthorized') { res.writeHead(401); res.end('denied'); return; }
        if (options.login === 'redirect') { res.writeHead(302, { location: '/capture' }); res.end(); return; }
        res.setHeader('set-cookie', `session=${cookie}; Path=/online; HttpOnly`);
        res.end(options.login === 'wrong-page' ? '<html>unrelated</html>' :
          `<form><input name="email"><input name="password">${options.login === 'missing-token' ? '' : `<input name="csrf_token" value="${token}">`}</form>`);
      } else if (url.pathname === '/online/index.php' && req.method === 'POST') {
        assert.equal(request.form.get('email'), email);
        assert.equal(request.form.get('password'), password);
        assert.equal(request.form.get('csrf_token'), token);
        assert.equal(req.headers.cookie, `session=${cookie}`);
        res.end(options.login === 'reject' ? '<input name="email"><input name="password">' :
          'messageType:"logininfo" app_my_class_schedule.php');
      } else if (url.pathname === '/online/class_calendar-ajax.php' && req.method === 'POST') {
        assert.equal(req.headers.accept, 'application/json');
        assert.equal(req.headers['x-csrf-token'], token);
        assert.equal(req.headers.cookie, `session=${cookie}`);
        assert.equal(request.form.get('action'), 'getclasses');
        assert.equal(request.form.get('selected_view'), 'agendaWeek');
        if (options.status) { res.writeHead(options.status); res.end('fixture error'); return; }
        // This provider labels JSON as HTML. The content remains the contract.
        res.setHeader('content-type', 'text/html');
        res.end(typeof options.schedule === 'string' ? options.schedule : JSON.stringify(options.schedule === undefined ? [] : options.schedule));
      } else if (url.pathname === '/online/my_payments.php' && req.method === 'GET') {
        assert.equal(req.headers.accept, 'text/html');
        assert.equal(req.headers.cookie, `session=${cookie}`);
        if (options.status) { res.writeHead(options.status); res.end('fixture error'); return; }
        res.end(options.payments ?? paymentsPage);
      } else if (url.pathname === '/online/my_history.php') {
        assert.equal(req.headers.accept, 'text/html');
        assert.equal(req.headers.cookie, `session=${cookie}`);
        let from = ['2025', '12', '01'];
        if (req.method === 'POST') {
          assert.equal(req.headers['x-csrf-token'], token);
          assert.equal(request.form.get('csrf_token'), token);
          assert.equal(request.form.get('btn_search'), 'Search');
          from = ['show_year', 'show_month', 'show_day'].map(k => request.form.get(k)!);
        } else assert.equal(req.method, 'GET');
        if (options.status) { res.writeHead(options.status); res.end('fixture error'); return; }
        res.end(options.history ?? historyPage(from));
      } else { throw new Error(`Unexpected request: ${req.method} ${req.url}`); }
    } catch (error) { failures.push(error as Error); res.writeHead(500); res.end('fixture assertion failed'); }
  };
  const portal = http.createServer(handler(false));
  const mobile = https.createServer({ key, cert: certificate }, handler(true));
  const proxy = http.createServer((_req, res) => { res.writeHead(502); res.end(); });
  const portalPort = await listen(portal);
  const mobilePort = await listen(mobile);
  const sockets = new Set<net.Socket>();
  proxy.on('connect', (req, socket, head) => {
    assert.equal(req.url, 'mobileinventor.com:443', 'external network access is forbidden');
    const upstream = net.connect(mobilePort, '127.0.0.1', () => {
      socket.write('HTTP/1.1 200 Connection Established\r\n\r\n');
      if (head.length) upstream.write(head);
      socket.pipe(upstream); upstream.pipe(socket);
    });
    sockets.add(socket); sockets.add(upstream);
    socket.on('error', () => upstream.destroy()); upstream.on('error', () => socket.destroy());
  });
  const proxyPort = await listen(proxy);
  const baseURL = `http://127.0.0.1:${portalPort}/online`;
  try {
    const run = (args: string[], extra: Record<string, string> = {}, input = '') => new Promise<Result>((resolve, reject) => {
      const child = spawn(binary, ['--config', config, ...args], {
        env: { PATH: process.env.PATH, HOME: home, XDG_CONFIG_HOME: join(home, '.config'), TERM: 'dumb', TZ: 'UTC',
          EDCCTL_BASE_URL: baseURL, EDCCTL_USERNAME: email, EDCCTL_PASSWORD: password,
          HTTPS_PROXY: `http://127.0.0.1:${proxyPort}`, NO_PROXY: '127.0.0.1,localhost',
          SSL_CERT_FILE: join(suiteDir, 'cert.pem'), SSL_CERT_DIR: home,
          GODEBUG: 'x509sslcertoverrideplatform=1', ...extra },
        stdio: ['pipe', 'pipe', 'pipe'],
      });
      let stdout = '', stderr = '';
      const timer = setTimeout(() => { child.kill('SIGKILL'); reject(new Error(`CLI timeout: ${args.join(' ')}`)); }, 10_000);
      child.stdout.on('data', data => { stdout += data; });
      child.stderr.on('data', data => { stderr += data; });
      child.on('error', error => { clearTimeout(timer); reject(error); });
      child.on('close', code => { clearTimeout(timer); resolve({ code, stdout, stderr }); });
      child.stdin.end(input);
    });
    await body({ run, requests, home, config, baseURL });
    assert.deepEqual(failures, [], 'HTTP fixture contract failed');
  } finally {
    for (const socket of sockets) socket.destroy();
    for (const server of [portal, mobile, proxy]) {
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
    }
    await rm(home, { recursive: true, force: true });
  }
}

function ok(result: Result) {
  assert.equal(result.code, 0, result.stderr);
  for (const secret of [password, token, cookie, 'hidden-secret']) {
    assert.ok(!(result.stdout + result.stderr).includes(secret), `output leaks ${secret}`);
  }
  return result.stdout;
}
function failed(result: Result, code: number, message: RegExp) {
  assert.equal(result.code, code, result.stderr);
  assert.equal(result.stdout, '', 'failed command emits primary data');
  assert.match(result.stderr, message);
  for (const secret of [password, token, cookie]) assert.ok(!result.stderr.includes(secret));
}

// Synthetic copies of the verified portal markup. There is no JSON endpoint for these pages.
const paymentsPage = `<h3>You owe $1,234.50</h3><table class="table table-bordered"><tr><th>Name</th><th>Balance</th><th>Statement</th></tr>
<tr><td>Student One</td><td>Owes $1,250.00</td><td><a href="app_statements.php?sid=101">View</a></td></tr>
<tr><td>Student Two</td><td>($15.50)</td><td><a href="app_statements.php?sid=102">View</a></td></tr></table>`;
const longMemo = 'Registration - Tuition Due (Ballet II/III, Jazz/Lyrical/Contemporary Technique II/III, Tap II)';
const historyPage = ([year, month, day]: string[]) => `<h3>Payment History</h3>Balance: Owe $1,234.50<br>
<form method="post"><select name="show_month"><option selected value="${month}">M</option></select>
<select name="show_day"><option selected value="${day}">D</option></select><select name="show_year"><option selected value="${year}">Y</option></select></form>
<table class="table table-bordered"><tr><th>--</th><th>Pmts</th><th>Chrgs</th><th>Bal</th></tr>
<tr><td><span id='sp_9'>Registration<a>...</a></span><span id='sp_full_9' style='display:none'>${longMemo}</span><br><p class="text-info"><i>Student One</i></p><p><small>Nov 15,2026</small></p></td><td></td><td>1,250.00</td><td>$1,234.50</td></tr>
<tr><td>October Tuition -- <br><p class="text-info"><i>Student One</i></p><p><small>Oct 01,2026</small></p></td><td>20.50</td><td></td><td>($15.50)</td></tr>
<tr><!--<td></td>--><td></td><td></td><td></td><td>$5.00</td></tr></table>`;

const group = (id: number, rules = {}) => ({ id, appID: 2555, name: `Group ${id}`, description: 'studio',
  anyoneCanJoin: 1, invitationOnly: 0, askToJoin: 0, loginRequired: 0, hidden: 0, pwd: '', memberCnt: 3, ...rules });
const encode = (s: string) => Buffer.from(s).toString('base64');
function appResponse(r: Request) {
  switch (r.url.pathname) {
    case '/applib/getAppInfo.php':
      assert.equal(r.url.searchParams.get('appid'), '2555');
      return { appinfo: { appName: 'Dance Studio', appDesc: 'Dance', iosVersion: '1.2', androidVersion: '2.3',
        Itunes_Link: 'https://example.test/ios', Android_Link: 'https://example.test/android' },
        locations: [{ id: '1', name: 'Main', cmsID: '30834', cmsType: 'dancestudiopro' }], chattoken: 'hidden-secret' };
    case '/applib/groupsUtil.php':
      assert.equal(r.url.searchParams.get('req'), 'getgroups');
      assert.equal(r.url.searchParams.get('appID'), '2555');
      return [group(10), group(11, { pwd: 'hidden-secret' }), group(12, { hidden: 1 })];
    case '/applib/notificationsUtil.php':
      assert.equal(r.url.searchParams.get('req'), 'getMsgs');
      assert.equal(r.url.searchParams.get('appID'), '2555');
      assert.equal(r.url.searchParams.get('groupID'), '10');
      return { messages: [{ PushMsgId: 5, sentDt: 1790000000, title: 'Recital', Msg: 'Bring shoes\nDoors\topen\u001b',
        detail: encode('<b>Details</b>'), buttonText: 'More', pushType: 'link', PushPage: encode('https://example.test/recital'),
        mediaType: 'image', mediaSrc: encode('https://example.test/photo') }] };
    case '/applib/AppProfileManager.php':
      assert.equal(r.method, 'POST');
      assert.deepEqual(Object.fromEntries(r.form), { aid: '2555', email, req: 'getacctinfo' });
      return { success: true, data: { id: '42', firstName: 'Test', lastName: 'Parent', email, chattoken: 'hidden-secret' } };
    default: throw new Error(`Unexpected app request ${r.url.pathname}`);
  }
}

test('config persists stdin credentials, protects overwrite, redacts output, and honors environment priority', async () => {
  await fixture({}, async ({ run, config, baseURL, requests }) => {
    const receipt = JSON.parse(ok(await run(['--json', 'config', 'init', '--email', email, '--password-stdin', '--base-url', baseURL], {}, password + '\n')));
    assert.deepEqual(receipt, { path: config, status: 'written' });
    assert.equal((await stat(config)).mode & 0o777, 0o600);
    const saved = await readFile(config, 'utf8');
    assert.match(saved, /password: fixture-password/);
    failed(await run(['config', 'init']), 1, /already exists/);
    assert.equal(await readFile(config, 'utf8'), saved);
    const shown = JSON.parse(ok(await run(['--json', 'config', 'show'], { EDCCTL_USERNAME: 'override@example.test', EDC_LOGIN: 'fallback@example.test' })));
    assert.equal(shown.email, 'override@example.test');
    assert.equal(shown.password, 'redacted');
    assert.equal(shown.path, config);
    assert.ok(ok(await run(['--plain', 'config', 'show'])).includes(config));
    assert.ok(ok(await run(['config', 'show'])).includes(config));
    assert.equal(requests.length, 0);
    const login = JSON.parse(ok(await run(['--json', 'login'], { EDCCTL_USERNAME: '', EDCCTL_PASSWORD: '' })));
    assert.equal(login.email, email);
  });
});

test('forced config replacement preserves linked files and private permissions', async () => {
  await fixture({}, async ({ run, config, home }) => {
    const original = 'email: old@example.test\n';
    await writeFile(config, original, { mode: 0o644 });
    await chmod(config, 0o644);
    const oldConfig = join(home, 'old.yaml');
    await link(config, oldConfig);
    ok(await run(['config', 'init', '--force', '--password-stdin'], {}, password));
    assert.equal((await stat(config)).mode & 0o777, 0o600);
    assert.match(await readFile(config, 'utf8'), /password: fixture-password/);
    assert.equal(await readFile(oldConfig, 'utf8'), original);
    assert.equal((await stat(oldConfig)).mode & 0o777, 0o644);
  });
});

test('config init refuses live and dangling destination symlinks', async () => {
  await fixture({}, async ({ run, config, home, requests }) => {
    const target = join(home, 'target.yaml');
    const original = 'email: old@example.test\n';
    await writeFile(target, original, { mode: 0o644 });
    await chmod(target, 0o644);
    await symlink(target, config);
    for (const dangling of [false, true]) {
      if (dangling) await rm(target);
      for (const flags of [[], ['--force']]) {
        failed(await run(['config', 'init', '--password-stdin', ...flags], {}, password), 1, /already exists|non-regular/);
        assert.ok((await lstat(config)).isSymbolicLink());
        if (dangling) {
          await assert.rejects(stat(target), { code: 'ENOENT' });
        } else {
          assert.equal(await readFile(target, 'utf8'), original);
          assert.equal((await stat(target)).mode & 0o777, 0o644);
        }
      }
    }
    assert.equal(requests.length, 0);
  });
});

test('dry-run config init refuses persistent changes', async () => {
  await fixture({}, async ({ run, config }) => {
    failed(await run(['--dry-run', 'config', 'init', '--password-stdin'], {}, password), 1, /dry-run/);
    await assert.rejects(stat(config), { code: 'ENOENT' });
    const original = 'email: old@example.test\n';
    await writeFile(config, original, { mode: 0o644 });
    await chmod(config, 0o644);
    failed(await run(['--dry-run', 'config', 'init', '--force', '--password-stdin'], {}, password), 1, /dry-run/);
    assert.equal(await readFile(config, 'utf8'), original);
    assert.equal((await stat(config)).mode & 0o777, 0o644);
  });
});

test('format conflicts fail for network and client-free commands', async () => {
  await fixture({}, async ({ run, requests }) => {
    for (const command of [['--version'], ['version'], ['config'], ['config', 'show'], ['config', 'init', '--force'], ['completion', 'bash'], ['login'], ['app'], ['app', 'info']]) {
      failed(await run(['--json', '--plain', ...command]), 2, /choose only one/);
    }
    assert.equal(requests.length, 0);
  });
});

test('config parse/read errors and missing credentials fail before network access', async () => {
  await fixture({}, async ({ run, config, requests }) => {
    failed(await run(['login'], { EDCCTL_USERNAME: '' }), 1, /email is required.*EDCCTL_USERNAME/);
    failed(await run(['login'], { EDCCTL_PASSWORD: '' }), 1, /password is required.*EDCCTL_PASSWORD/);
    await writeFile(config, 'base_url: [broken');
    failed(await run(['--json', 'config', 'show']), 1, /parse config file/);
    await rm(config);
    await writeFile(config, 'base_url: [broken');
    failed(await run(['login']), 1, /parse config file/);
    assert.equal(requests.length, 0);
  });
});

test('login fallback credentials, trace, account flag override, and output modes', async () => {
  await fixture({}, async ({ run, requests }) => {
    const result = await run(['--trace-http', '--account-id', '123', '--json', 'login'], {
      EDCCTL_USERNAME: '', EDCCTL_PASSWORD: '', EDC_LOGIN: email, EDC_PASSWORD: password,
    });
    assert.equal(JSON.parse(ok(result)).account_id, '123');
    assert.match(result.stderr, /\[http\] GET \/online\/index.php -> 200/);
    assert.match(result.stderr, /\[http\] POST \/online\/index.php -> 200/);
    assert.ok(!result.stderr.includes('account_id'));
    assert.equal(requests[0].url.searchParams.get('account_id'), '123');
    assert.match(ok(await run(['--no-color', 'login'])), /\[ok\] login successful/);
    assert.equal(ok(await run(['--quiet', 'login'])), '');
  });
});

for (const [mode, message] of [
  ['reject', /login failed/], ['missing-token', /CSRF token/], ['wrong-page', /expected login form/],
  ['redirect', /redirect refused/], ['unauthorized', /HTTP 401/], ['slow', /deadline exceeded|Client.Timeout/],
] as const) {
  test(`login rejects ${mode} without data queries or leaked credentials`, async () => {
    await fixture({ login: mode }, async ({ run, requests }) => {
      failed(await run(['--timeout', '100ms', '--json', 'schedule']), 1, message);
      assert.ok(requests.every(r => r.url.pathname === '/online/index.php'));
    });
  });
}

test('transport failure and dry-run login exit without data or POST', async () => {
  await fixture({}, async ({ run, requests }) => {
    failed(await run(['--base-url', 'http://127.0.0.1:1/online', '--timeout', '100ms', 'login']), 1, /request failed/);
    failed(await run(['--dry-run', 'schedule']), 1, /dry-run: refusing POST/);
    assert.equal(requests.length, 1);
    assert.equal(requests[0].method, 'GET');
  });
});

for (const boundary of ['untrusted certificate', 'unreachable CONNECT proxy'] as const) {
  test(`app refuses ${boundary} before app HTTP or credential disclosure`, async () => {
    await fixture({}, async ({ run, requests, home }) => {
      const extra = boundary === 'untrusted certificate'
        ? { SSL_CERT_FILE: join(home, 'untrusted.pem') }
        : { HTTPS_PROXY: 'http://127.0.0.1:1' };
      failed(await run(['--timeout', '200ms', '--json', 'app', 'info'], extra), 1,
        boundary === 'untrusted certificate' ? /certificate|unknown authority/ : /connection refused|proxyconnect/);
      assert.equal(requests.filter(r => r.app).length, 0, 'app HTTP or credentials reach the mobile server after a transport failure');
    });
  });
}

test('balance reads per-student balances from the portal page in JSON, plain, and table output', async () => {
  await fixture({}, async ({ run, requests }) => {
    assert.deepEqual(JSON.parse(ok(await run(['--json', 'balance']))), { balance_cents: 123450, students: [
      { student_id: '101', name: 'Student One', balance_cents: 125000 },
      { student_id: '102', name: 'Student Two', balance_cents: -1550 }] });
    assert.equal(ok(await run(['--plain', 'balance'])),
      '101\tStudent One\t$1250.00\n102\tStudent Two\t-$15.50\n\tTotal\t$1234.50\n');
    assert.match(ok(await run(['balance'])), /STUDENT ID\s+NAME\s+BALANCE[\s\S]*Total\s+\$1234\.50/);
    assert.ok(requests.every(r => r.method === 'GET' || r.url.pathname === '/online/index.php'));
  });
});

test('history reads the ledger by default GET or date search POST, with full JSON and plain memos', async () => {
  await fixture({}, async ({ run, requests }) => {
    const history = JSON.parse(ok(await run(['--json', 'history'])));
    assert.deepEqual(history, { from: '2025-12-01', balance_cents: 123450, opening_balance_cents: 500, entries: [
      { date: '2026-11-15', student: 'Student One', description: longMemo, payment_cents: 0, charge_cents: 125000, balance_cents: 123450 },
      { date: '2026-10-01', student: 'Student One', description: 'October Tuition', payment_cents: 2050, charge_cents: 0, balance_cents: -1550 }] });
    assert.equal(requests.at(-1)!.method, 'GET');
    assert.equal(JSON.parse(ok(await run(['--json', 'history', '--from', '2026-09-01']))).from, '2026-09-01');
    assert.equal(requests.at(-1)!.method, 'POST');
    const plain = ok(await run(['--plain', 'history'])).trimEnd().split('\n');
    assert.deepEqual(plain, [`2026-11-15\tStudent One\t${longMemo}\t\t$1250.00\t$1234.50`,
      '2026-10-01\tStudent One\tOctober Tuition\t$20.50\t\t-$15.50']);
    const table = ok(await run(['history']));
    assert.match(table, /^Since 2025-12-01; balance \$1234\.50; opening balance \$5\.00\n/);
    assert.match(table, /DATE\s+STUDENT\s+DESCRIPTION\s+PAYMENT\s+CHARGE\s+BALANCE/);
    assert.ok(table.includes('…') && !table.includes(longMemo), 'table truncates long memos');
  });
});

for (const [name, options, args, message] of [
  ['unreconciled ledger', { history: historyPage(['2025', '12', '01']).replace('<td>20.50</td>', '<td>20.00</td>') }, ['history'], /do not reconcile/],
  ['changed ledger header', { history: historyPage(['2025', '12', '01']).replace('Pmts', 'Payments') }, ['history'], /unexpected structure/],
  ['expired session', { history: '<form><input name="email"><input name="password"></form>' }, ['history'], /session may have expired/],
  ['mismatched balance heading', { payments: paymentsPage.replace('You owe $1,234.50', 'You owe $1.00') }, ['balance'], /does not match/],
  ['unknown amount format', { payments: paymentsPage.replace('Owes', 'Due') }, ['balance'], /unexpected structure/],
] as const) {
  test(`${args[0]} refuses ${name} rather than partial data`, async () => {
    await fixture(options, async ({ run }) => { failed(await run(['--json', ...args]), 1, message); });
  });
}

test('history refuses a page for another start date', async () => {
  await fixture({ history: historyPage(['2025', '12', '01']) }, async ({ run }) => {
    failed(await run(['--json', 'history', '--from', '2026-09-01']), 1, /not the requested 2026-09-01/);
  });
});

const range = ['--from', '2026-09-28', '--to', '2026-10-05'];
const event = (id: string, start: string, title = 'Ballet') => ({ id, type: 'class', sid: '7', title, start, end: start });
test('schedule sorts and filters exclusive range in JSON, plain, and table output', async () => {
  await fixture({ schedule: [event('outside', '2026-10-05'), event('late', '2026-09-30T18:00:00'),
    event('early', '2026-09-28'), event('before', '2026-09-27T23:59:59')] }, async ({ run, requests }) => {
    const events = JSON.parse(ok(await run(['--json', 'schedule', ...range])));
    assert.deepEqual(events.map((e: { id: string }) => e.id), ['early', 'late']);
    const plain = ok(await run(['--plain', 'schedule', ...range])).trimEnd().split('\n');
    assert.equal(plain.length, 2);
    assert.equal(plain[0], 'early\tclass\t7\tBallet\t2026-09-28\t2026-09-28');
    const table = ok(await run(['schedule', ...range]));
    assert.match(table, /START\s+END\s+TITLE/);
    assert.match(table, /Ballet/);
    assert.ok(!table.includes('2026-10-05'));
    for (const r of requests.filter(r => r.url.pathname.includes('calendar'))) {
      assert.equal(r.form.get('start'), '2026-09-28'); assert.equal(r.form.get('end'), '2026-10-05');
    }
  });
});

test('empty schedule and doctor use the Monday week with offset support', async () => {
  await fixture({}, async ({ run, requests }) => {
    assert.deepEqual(JSON.parse(ok(await run(['--json', 'schedule', '--week', '2']))), []);
    assert.equal(ok(await run(['--plain', 'schedule', ...range])), '');
    const doctor = JSON.parse(ok(await run(['--json', 'doctor'])));
    assert.equal(doctor.ok, true); assert.equal(doctor.events, 0); assert.equal(doctor.endpoint, '/class_calendar-ajax.php');
    assert.match(ok(await run(['doctor'])), /JSON schedule endpoint reachable/);
    const calendars = requests.filter(r => r.url.pathname.includes('calendar'));
    const current = new Date(calendars[2].form.get('start') + 'T00:00:00Z');
    assert.equal(current.getUTCDay(), 1);
    const offset = new Date(calendars[0].form.get('start') + 'T00:00:00Z');
    assert.equal(+offset - +current, 14 * 86400000);
    for (const r of [calendars[0], calendars[2]]) {
      assert.equal(+new Date(r.form.get('end')!) - +new Date(r.form.get('start')!), 7 * 86400000);
    }
  });
});

for (const [name, schedule] of [
  ['HTML', '<html>expired</html>'], ['malformed', '[broken'], ['null', null],
  ['object', {}], ['invalid event start', [event('bad', 'nonsense')]],
] as const) {
  test(`schedule refuses ${name} response rather than partial data`, async () => {
    await fixture({ schedule }, async ({ run, requests }) => {
      failed(await run(['--json', 'schedule', ...range]), 1, /JSON event array|invalid event start/);
      assert.equal(requests.length, 3);
    });
  });
}

test('usage validation prevents login for invalid schedule and app arguments', async () => {
  await fixture({}, async ({ run, requests }) => {
    for (const args of [
      ['schedule', '--from', '2026-09-28'], ['schedule', '--to', '2026-10-05'],
      ['schedule', '--from', 'bad', '--to', '2026-10-05'], ['schedule', '--from', '2026-10-05', '--to', '2026-09-28'],
      ['schedule', ...range, '--week', '0'], ['schedule', 'extra'], ['doctor', 'extra'],
      ['balance', 'extra'], ['history', 'extra'], ['history', '--from', '2026-9-1'],
      ['app', 'info', 'extra'], ['app', 'profile', 'other@example.test'],
      ['app', 'notifications'], ['app', 'notifications', '--group', '-1'],
    ]) failed(await run(args), 2, /invalid usage/);
    assert.equal(requests.length, 0);
  });
});

test('app resources use actual HTTPS, independent session, selected fields, and stable output modes', async () => {
  await fixture({}, async ({ run, requests }) => {
    for (const resource of ['info', 'groups', 'notifications', 'profile']) {
      const args = ['app', resource, ...(resource === 'notifications' ? ['--group', '10'] : [])];
      const json = JSON.parse(ok(await run(['--json', '--trace-http', ...args])));
      if (resource === 'info') { assert.equal(json.name, 'Dance Studio'); assert.equal(json.locations[0].cmsID, '30834'); }
      if (resource === 'groups') { assert.deepEqual(json.map((g: { id: number }) => g.id), [10, 11]); assert.equal(json[1].public, false); }
      if (resource === 'notifications') {
        assert.equal(json[0].detail_html, '<b>Details</b>'); assert.equal(json[0].link, 'https://example.test/recital');
      }
      if (resource === 'profile') { assert.equal(json.email, email); assert.equal(json.id, '42'); }
      const plain = ok(await run(['--plain', ...args]));
      assert.ok(plain.includes('\t')); assert.ok(!plain.includes('\x1b'));
      if (resource === 'notifications') {
        assert.equal(plain.trimEnd().split('\n').length, 1); assert.equal(plain.trimEnd().split('\t').length, 4);
      }
      const table = ok(await run(['--no-color', ...args]));
      assert.match(table, resource === 'groups' ? /PUBLIC/ : resource === 'notifications' ? /SENT UTC/ : /FIELD/);
    }
    assert.equal(requests.filter(r => r.app).length, 15);
    assert.ok(requests.every(r => r.url.hostname === 'fixture.test'));
  });
});

test('restricted and unknown notification groups never reach message endpoint', async () => {
  const groups = [group(10), group(11, { pwd: 'hidden-secret' }), group(12, { invitationOnly: 1 }),
    group(13, { askToJoin: 1 }), group(14, { loginRequired: 1 }), group(15, { hidden: 1 }), group(16, { anyoneCanJoin: 0 })];
  await fixture({ app: { '/applib/groupsUtil.php': groups } }, async ({ run, requests }) => {
    for (const id of [11, 12, 13, 14, 15, 16, 999]) {
      failed(await run(['--json', 'app', 'notifications', '--group', String(id)]), 1, /unknown or restricted/);
    }
    assert.ok(!requests.some(r => r.url.pathname.includes('notificationsUtil')));
  });
});

for (const [name, resource, path, payload, message] of [
  ['incomplete info', 'info', '/applib/getAppInfo.php', { appinfo: {} }, /incomplete/],
  ['missing access rules', 'groups', '/applib/groupsUtil.php', [{ id: 10, name: 'Unsafe', appID: 2555 }], /access rules/],
  ['wrong app identity', 'groups', '/applib/groupsUtil.php', [group(10, { appID: 999 })], /identity/],
  ['null', 'groups', '/applib/groupsUtil.php', null, /expected JSON/],
  ['missing messages', 'notifications', '/applib/notificationsUtil.php', {}, /message array/],
  ['invalid base64', 'notifications', '/applib/notificationsUtil.php', { messages: [{ PushMsgId: 5, sentDt: 1790000000, detail: '!', PushPage: '', mediaSrc: '' }] }, /invalid fields/],
  ['wrong profile identity', 'profile', '/applib/AppProfileManager.php', { success: true, data: { id: '42', email: 'other@example.test' } }, /authenticated email/],
  ['unavailable profile', 'profile', '/applib/AppProfileManager.php', { success: false }, /authenticated email/],
] as const) {
  test(`app refuses ${name}`, async () => {
    await fixture({ app: { [path]: payload } }, async ({ run }) => {
      failed(await run(['--json', 'app', resource, ...(resource === 'notifications' ? ['--group', '10'] : [])]), 1, message);
    });
  });
}

test('empty app collections return arrays and non-EDC account cannot query app', async () => {
  await fixture({ app: { '/applib/groupsUtil.php': [], '/applib/notificationsUtil.php': { messages: [] } } }, async ({ run, requests }) => {
    assert.deepEqual(JSON.parse(ok(await run(['--json', 'app', 'groups']))), []);
    const count = requests.filter(r => r.app).length;
    failed(await run(['--account-id', '999', 'app', 'info']), 1, /only EDC account/);
    assert.equal(requests.filter(r => r.app).length, count);
  });
  await fixture({ app: { '/applib/notificationsUtil.php': { messages: [] } } }, async ({ run }) => {
    assert.deepEqual(JSON.parse(ok(await run(['--json', 'app', 'notifications', '--group', '10']))), []);
  });
});

test('data HTTP errors emit diagnostics rather than empty success', async () => {
  await fixture({ status: 503 }, async ({ run }) => {
    for (const args of [['schedule', ...range], ['doctor'], ['balance'], ['history'], ['app', 'info']]) {
      failed(await run(['--json', ...args]), 1, /HTTP 503/);
    }
  });
});

test('version and completion run without credentials, with valid shell scripts and bad shell rejection', async () => {
  await fixture({}, async ({ run, requests, config }) => {
    const env = { EDCCTL_USERNAME: '', EDCCTL_PASSWORD: '' };
    await writeFile(config, 'base_url: [broken');
    const info = JSON.parse(ok(await run(['--json', 'version'], env)));
    assert.equal(typeof info.version, 'string');
    assert.ok(info.version.length > 0);
    assert.deepEqual(info, { version: info.version, commit: buildCommit, date: buildDate });
    assert.equal(ok(await run(['--version'], env)), `edcctl version ${info.version}\ncommit: ${info.commit}\nbuilt:  ${info.date}\n`);
    assert.equal(ok(await run(['--plain', 'version'], env)), `${info.version}\n`);
    for (const [shell, marker] of [['bash', /complete.*edcctl/], ['zsh', /compdef.*edcctl/],
      ['fish', /complete.*edcctl/], ['powershell', /Register-ArgumentCompleter/]] as const) {
      assert.match(ok(await run(['completion', shell], env)), marker);
    }
    failed(await run(['completion', 'unsupported'], env), 2, /unsupported shell/);
    failed(await run(['completion'], env), 2, /invalid usage/);
    for (const removed of ['students', 'announcements', 'files', 'account']) {
      failed(await run([removed], env), 2, /unknown command/);
    }
    assert.equal(requests.length, 0);
  });
});

test('unknown commands, flags, and extra arguments return usage code before config or login', async () => {
  await fixture({}, async ({ run, requests, config }) => {
    await writeFile(config, 'base_url: [broken');
    for (const args of [
      ['unknown-command'], ['--unknown'], ['version', '--unknown'], ['version', '--json=bad'],
      ['version', 'extra'], ['--version', 'extra'], ['config', 'show', 'extra'], ['config', 'init', 'extra'],
      ['config', 'unknown-command'], ['app', 'unknown-command'], ['login', 'extra'],
      ['login', '--version'], ['app', 'info', '--version'], ['completion', 'bash', 'extra'],
    ]) failed(await run(args), 2, /invalid usage/);
    assert.equal(requests.length, 0);
  });
});
