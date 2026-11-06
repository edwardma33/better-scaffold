import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';

const [app, tooling] = process.argv.slice(2);
if (!app || !tooling) throw new Error('Usage: node scripts/auth-smoke.mjs <generated app> <tooling directory>');
const require = createRequire(path.join(tooling, 'package.json'));
const { chromium } = require('playwright');
const server = spawn('npm', ['run', 'start', '--', '--port', '3000'], {
  cwd: app, stdio: 'inherit', detached: process.platform !== 'win32',
  env: { ...process.env, NEXT_TELEMETRY_DISABLED: '1' },
});
const stopped = new Promise(resolve => server.once('exit', (code, signal) => resolve({ code, signal })));
let serverExited = false;
server.once('exit', () => { serverExited = true; });
const terminate = () => {
  if (serverExited) return;
  if (process.platform === 'win32') server.kill('SIGTERM');
  else { try { process.kill(-server.pid, 'SIGTERM'); } catch (error) { if (error.code !== 'ESRCH') throw error; } }
};
const abort = () => { terminate(); };
process.on('SIGINT', abort);
process.on('SIGTERM', abort);
let browser;
let page;
const artifacts = process.env.INTEGRATION_ARTIFACTS || path.join(app, 'integration-artifacts');
try {
  const deadline = Date.now() + 90_000;
  let ready = false;
  while (Date.now() < deadline) {
    if (serverExited) throw new Error('Next.js exited before becoming ready');
    try { ready = (await fetch('http://localhost:3000/sign-in', { signal: AbortSignal.timeout(3000) })).ok; } catch {}
    if (ready) break;
    await delay(500);
  }
  assert.ok(ready, 'Next.js did not become ready within 90 seconds');
  browser = await chromium.launch({ headless: true });
  page = await browser.newPage();
  page.setDefaultTimeout(20_000);
  await page.goto('http://localhost:3000/dashboard');
  await page.waitForURL('**/sign-in');
  const email = `release-${Date.now()}@example.com`;
  const password = 'ReleaseCheck-Password-123';
  await page.goto('http://localhost:3000/sign-up');
  await page.getByLabel('Name', { exact: true }).fill('Release Check');
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Create account', exact: true }).click();
  await page.waitForURL('**/dashboard');
  assert.ok(await page.getByText(`Signed in as ${email}.`, { exact: true }).isVisible());
  await page.reload();
  await page.waitForURL('**/dashboard');
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  await page.waitForURL('**/sign-in');
  await page.goto('http://localhost:3000/dashboard');
  await page.waitForURL('**/sign-in');
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.waitForURL('**/dashboard');
  assert.ok(await page.getByText(`Signed in as ${email}.`, { exact: true }).isVisible());
  console.log('Verified sign-up, persisted session, sign-out, dashboard protection, and sign-in.');
} finally {
  if (page) { await mkdir(artifacts, { recursive: true }); await page.screenshot({ path: path.join(artifacts, 'authentication.png'), fullPage: true }).catch(() => {}); }
  await browser?.close();
  terminate();
  await Promise.race([stopped, delay(5000)]);
  if (!serverExited && process.platform !== 'win32') { try { process.kill(-server.pid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') throw error; } }
  process.removeListener('SIGINT', abort);
  process.removeListener('SIGTERM', abort);
}
