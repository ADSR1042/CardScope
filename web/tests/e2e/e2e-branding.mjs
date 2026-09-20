import { runtimePath } from './paths.mjs';
import { chromium } from '@playwright/test';
import fs from 'node:fs';
const access = JSON.parse(fs.readFileSync(runtimePath('access.json'), 'utf8'));
const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1050 } });
page.setDefaultTimeout(10000);
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));
const assert = (v, m) => {
  if (!v) throw Error(m);
};
let original;
async function login() {
  await page.goto(access.url);
  await page.waitForSelector('.login-form,nav');
  if (await page.getByRole('button', { name: '节点管理', exact: true }).count()) return;
  await page.getByLabel('账号', { exact: true }).fill('admin');
  await page.getByLabel('密码', { exact: true }).fill(access.admin);
  await page.getByRole('button', { name: '进入面板', exact: true }).click();
  await page.getByRole('button', { name: '节点管理', exact: true }).waitFor();
}
try {
  console.log('Login');
  await login();
  assert(
    (await page.locator('.workspace,.agent-note').count()) === 0,
    'Decorative sidebar blocks remain',
  );
  original = await page.evaluate(() => fetch('/api/v1/login-copy').then((r) => r.json()));
  await page.getByRole('button', { name: '节点管理', exact: true }).click();

  await page.getByLabel('主标题', { exact: true }).fill('实验室 GPU\n资源监控');
  await page
    .getByLabel('说明文字', { exact: true })
    .fill('<script>alert(1)</script>\n共享计算资源');
  await page.getByLabel('底部文字', { exact: true }).fill('');
  console.log('Saving copy');
  await page.getByRole('button', { name: '保存登录页文案' }).click();
  await page.getByRole('status').getByText('已保存', { exact: true }).waitFor();
  console.log('Reload');
  await page.reload();
  await page.getByRole('button', { name: '节点管理', exact: true }).click();
  await page.getByLabel('主标题', { exact: true }).waitFor();
  assert(
    (await page.getByLabel('主标题', { exact: true }).inputValue()) === '实验室 GPU\n资源监控',
    'Copy not persisted',
  );
  await page.screenshot({ path: runtimePath('settings-branding.png'), fullPage: true });
  console.log('Logout');
  await page.getByRole('button', { name: '退出登录' }).click();
  await page.locator('.login-art h1').filter({ hasText: '实验室 GPU' }).waitFor();
  assert(
    (await page.locator('.login-art p').textContent()) ===
      '<script>alert(1)</script>\n共享计算资源',
    'Plain text mismatch',
  );
  assert((await page.locator('.login-caption').textContent()) === '', 'Blank caption not hidden');
  await page.screenshot({ path: runtimePath('login-branding.png'), fullPage: true });
  assert(errors.length === 0, errors.join('\n'));
  console.log(
    'Passed: sidebar cleanup, admin save, reload persistence, anonymous login copy, multiline/plain-text rendering, blank fields.',
  );
} finally {
  if (original) {
    console.log('Login');
    await login();
    await page.evaluate(async (original) => {
      const me = await fetch('/api/v1/me').then((r) => r.json());
      const r = await fetch('/api/v1/login-copy', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': me.csrf },
        body: JSON.stringify(original),
      });
      if (!r.ok) throw Error('Restore failed');
    }, original);
  }
  await browser.close();
}
