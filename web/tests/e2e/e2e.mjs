import { chromium } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { root } from './paths.mjs';
const access = JSON.parse(fs.readFileSync(path.join(root, '.runtime/access.json'), 'utf8'));
const browser = await chromium.launch({ channel: 'msedge', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1050 } });
const page = await context.newPage();
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));
async function assert(value, message) {
  if (!value) throw new Error(message);
}
await page.goto(access.url);
await page.getByLabel('账号', { exact: true }).fill('admin');
await page.getByLabel('密码', { exact: true }).fill(access.admin);
await page.getByRole('button', { name: '进入面板' }).click();
await page.getByRole('heading', { name: '资源总览', exact: true }).waitFor();
await page.getByText('GeForce GTX 1050', { exact: true }).waitFor();
await assert(
  (await page.getByText('信息受限', { exact: true }).count()) === 1,
  'WSL must show limited attribution',
);
await page.screenshot({ path: path.join(root, '.runtime/overview-light.png'), fullPage: true });
await page.getByRole('button', { name: '切换深色' }).click();
await page.screenshot({ path: path.join(root, '.runtime/overview-dark.png'), fullPage: true });
await page.getByRole('button', { name: '切换浅色' }).click();
await page.getByRole('button', { name: '系统详情', exact: true }).click();
await page.locator('.resource-details').waitFor();
await page.screenshot({ path: path.join(root, '.runtime/node-details.png'), fullPage: true });
await page.locator('.gpu-row').first().click();
await page.getByRole('dialog').waitFor();
await page.getByText('计算进程', { exact: false }).first().waitFor();
await page.screenshot({ path: path.join(root, '.runtime/gpu-detail.png'), fullPage: true });
await page.getByRole('button', { name: '关闭', exact: true }).click();
await page.getByRole('button', { name: '使用统计', exact: true }).click();
await page.getByRole('heading', { name: '使用统计', exact: true }).waitFor();
await page.screenshot({ path: path.join(root, '.runtime/statistics.png'), fullPage: true });
const downloadPromise = page.waitForEvent('download');
await page.getByRole('button', { name: '导出', exact: true }).click();
const download = await downloadPromise;
await download.saveAs(path.join(root, '.runtime/gpu-hours.csv'));
await page.getByRole('button', { name: '节点管理', exact: true }).click();
await page.getByText('账号密码', { exact: true }).waitFor();
await page.getByRole('button', { name: '保存配置', exact: true }).click();
await page.getByRole('button', { name: '添加节点', exact: true }).click();
await page.getByRole('dialog').waitFor();
await page.getByRole('button', { name: '关闭', exact: true }).click();
await page.getByRole('button', { name: '资源总览', exact: false }).click();
await page.getByLabel('搜索节点或用户').fill('no-match-should-be-empty');
await page.getByText('没有符合条件的 GPU').waitFor();
await page.getByLabel('搜索节点或用户').fill('');
await page.setViewportSize({ width: 390, height: 844 });
await page.screenshot({ path: path.join(root, '.runtime/mobile.png'), fullPage: true });
await assert(
  await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
  'Mobile page overflow',
);
await page.setViewportSize({ width: 1440, height: 1050 });
await page.getByRole('button', { name: '退出登录' }).click();
await page.getByRole('button', { name: '进入面板' }).waitFor();
await page.getByLabel('账号', { exact: true }).fill('viewer');
await page.getByLabel('密码', { exact: true }).fill(access.viewer);
await page.getByRole('button', { name: '进入面板' }).click();
await page.getByRole('heading', { name: '资源总览', exact: true }).waitFor();
await assert(
  (await page.getByRole('button', { name: '添加节点', exact: true }).count()) === 0,
  'viewer sees mutation',
);
await assert(
  (await page.getByRole('button', { name: '节点管理', exact: true }).count()) === 0,
  'viewer sees admin section',
);
await assert(errors.length === 0, 'Browser errors: ' + errors.join(';'));
fs.writeFileSync(
  path.join(root, '.runtime/browser-result.json'),
  JSON.stringify(
    {
      passed: true,
      checks: [
        'admin login',
        'real WSL GPU',
        'light/dark',
        'node details',
        'GPU drawer',
        'statistics',
        'CSV download',
        'settings save',
        'search empty state',
        'mobile overflow',
        'viewer permissions',
      ],
      pageErrors: errors,
    },
    null,
    2,
  ),
);
await browser.close();
console.log('Browser checks passed (12 scenarios); screenshots saved under .runtime.');
