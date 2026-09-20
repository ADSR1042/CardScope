import { chromium, expect } from '@playwright/test';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync } from 'node:fs';
import { once } from 'node:events';
import path from 'node:path';
import net from 'node:net';
import { root, runtimePath } from './paths.mjs';

// Every run uses a fresh database and loopback server; no existing access.json is read.
mkdirSync(runtimePath(''), { recursive: true });
const state = mkdtempSync(runtimePath('smoke-'));
const binary = path.join(state, process.platform === 'win32' ? 'gpu-hub.exe' : 'gpu-hub');
const build = spawnSync(
  process.env.GO_BINARY || 'go',
  ['build', '-buildvcs=false', '-o', binary, './cmd/gpu-hub'],
  {
    cwd: root,
    windowsHide: true,
    stdio: 'inherit',
  },
);
if (build.error) throw build.error;
if (build.status !== 0) throw Error('Go build failed');
const probe = net.createServer();
probe.listen(0, '127.0.0.1');
await once(probe, 'listening');
const port = probe.address().port;
await new Promise((resolve) => probe.close(resolve));
const url = `http://127.0.0.1:${port}`;
const server = spawn(binary, ['serve', '--listen', `127.0.0.1:${port}`, '--data', state], {
  windowsHide: true,
  stdio: ['pipe', 'pipe', 'pipe'],
});
server.stderr.resume();
server.stdin.on('error', () => {});
server.stdin.end('structure-admin-test\nstructure-viewer-test\n');
let browser;
try {
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(Error('Hub startup timeout')), 15000);
    server.stdout.on('data', (data) => {
      if (data.toString().includes('listening')) {
        clearTimeout(timer);
        resolve();
      }
    });
    server.on('error', reject);
    server.on('exit', (code) => {
      clearTimeout(timer);
      reject(Error(`Hub exited ${code}`));
    });
  });
  browser = await chromium.launch({ channel: 'msedge', headless: true });
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', (e) => errors.push(e.message));
  await page.goto(url);
  await page.getByLabel('账号', { exact: true }).fill('admin');
  await page.getByLabel('密码', { exact: true }).fill('structure-admin-test');
  await page.getByRole('button', { name: '进入面板', exact: true }).click();
  await page.getByRole('button', { name: '资源查询', exact: true }).click();
  await page.getByRole('heading', { name: '资源查询', exact: true }).waitFor();
  await page.getByRole('button', { name: '添加节点', exact: true }).first().click();
  await page.getByLabel('节点名称', { exact: true }).fill('Structure test node');
  await page.getByRole('button', { name: '生成接入码', exact: true }).click();
  await page.locator('.enrollment code').waitFor();
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: '节点管理', exact: true }).click();
  await page.getByLabel('主标题', { exact: true }).fill('Structure smoke test');
  await page.getByRole('button', { name: '保存登录页文案' }).click();
  await page.getByRole('status').getByText('已保存', { exact: true }).waitFor();
  await page.reload();
  await page.getByRole('button', { name: '节点管理', exact: true }).click();
  await expect(page.getByLabel('主标题', { exact: true })).toHaveValue('Structure smoke test');
  await page.getByRole('button', { name: '使用统计', exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: path.join(state, 'mobile.png'), fullPage: true });
  if (!(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)))
    throw Error('Mobile overflow');
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByRole('button', { name: '退出登录' }).click();
  await expect(page.locator('.login-art h1')).toHaveText('Structure smoke test');
  await page.getByLabel('账号', { exact: true }).fill('viewer');
  await page.getByLabel('密码', { exact: true }).fill('structure-viewer-test');
  await page.getByRole('button', { name: '进入面板', exact: true }).click();
  await page.getByRole('button', { name: '资源查询' }).click();
  await page.getByRole('heading', { name: '资源查询', exact: true }).waitFor();
  await expect(page.getByRole('button', { name: '添加节点', exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '节点管理', exact: true })).toHaveCount(0);
  if (errors.length) throw Error(errors.join('\n'));
  console.log(
    'PASS: embedded UI, login, CSRF node creation, settings persistence, statistics, mobile layout, logout and viewer permissions.',
  );
} finally {
  try {
    if (browser) await browser.close();
  } finally {
    if (server.pid && server.exitCode === null && server.signalCode === null) {
      const stopped = once(server, 'exit');
      server.kill();
      await stopped;
    }
    console.log(`Smoke artifacts: ${state}`);
  }
}
