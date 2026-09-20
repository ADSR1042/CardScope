import { runtimePath } from './paths.mjs';
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
const access = JSON.parse(fs.readFileSync(runtimePath('access.json'), 'utf8'));
const snapshot = JSON.parse(fs.readFileSync(runtimePath('client6000-doctor.json'), 'utf8'));
snapshot.gpus = snapshot.gpus.slice(0, 3).map((g, i) => ({
  ...g,
  index: i,
  power: i === 2 ? null : 100 - i * 50,
  status: 'ok',
  process_status: i === 2 ? 'permission_denied' : 'ok',
  util: i === 0 ? 0 : 70,
  memory_used: i === 0 ? 0 : 10 * 1024 ** 3,
  processes:
    i === 1
      ? [
          {
            pid: 123,
            uid: '1000',
            user: 'demo',
            name: 'python',
            created: 1,
            first_seen: 1,
            last_seen: 1,
            memory: 10 * 1024 ** 3,
          },
        ]
      : [],
}));
const node = {
  id: 'ui-test',
  name: '交互测试 · A6000',
  online: true,
  enrolled: true,
  expected: 3,
  networks: [],
  sample_at: Date.now(),
  received_at: Date.now(),
  snapshot,
};
const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
page.setDefaultTimeout(10000);
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));
await page.route('**/api/v1/nodes', (route) => route.fulfill({ json: [node] }));
try {
  await page.goto(access.url);
  await page.getByLabel('账号', { exact: true }).fill('viewer');
  await page.getByLabel('密码', { exact: true }).fill(access.viewer);
  await page.getByRole('button', { name: '进入面板', exact: true }).click();
  const card = page.locator('.node-card');
  const summary = card.locator('.node-summary');
  await expect(summary).toContainText('空闲 1');
  await expect(summary).toContainText('占用 1');
  await expect(summary).toContainText('未知 1');
  await expect(summary).toContainText('150 W');
  await expect(summary).toContainText('2/3 卡');
  await card.getByRole('button', { name: '收起', exact: true }).click();
  await expect(card.locator('.gpu-table')).toBeHidden();
  await expect(card.locator('.node-footer')).toBeHidden();
  await expect(summary).toBeVisible();
  await page.waitForResponse((r) => r.url().endsWith('/api/v1/nodes'));
  await expect(card.locator('.gpu-table')).toBeHidden();
  await card.getByRole('button', { name: '展开', exact: true }).click();
  await expect(card.locator('.gpu-table')).toBeVisible();
  await expect(card.locator('.node-expanded')).toHaveCount(0);
  await card.getByRole('button', { name: '系统详情', exact: true }).click();
  await expect(card.locator('.node-expanded')).toBeVisible();
  await expect(card.locator('.gpu-table')).toBeVisible();
  await expect(card.getByRole('progressbar', { name: 'CPU使用率', exact: true })).toBeVisible();
  await card.locator('.resource-disclosure summary').filter({ hasText: '设备 I/O' }).click();
  await card.locator('.resource-disclosure summary').filter({ hasText: '其他网卡' }).click();
  for (const selector of ['.io-list', '.other-interfaces']) {
    const size = await card.locator(selector).evaluate((e) => ({
      height: e.clientHeight,
      scroll: e.scrollHeight,
      overflow: getComputedStyle(e).overflowY,
    }));
    if (size.height > 280 || size.scroll <= size.height || size.overflow !== 'auto')
      throw Error('Unbounded list ' + selector + JSON.stringify(size));
  }
  await expect(card.locator('.primary-interfaces .interface-row').last()).toHaveCSS(
    'border-bottom-width',
    '0px',
  );
  await page.screenshot({ path: runtimePath('resources-scroll.png'), fullPage: true });
  await card.getByRole('button', { name: '收起详情', exact: true }).click();
  await expect(card.locator('.node-expanded')).toHaveCount(0);
  await page.getByRole('combobox', { name: '状态筛选', exact: true }).click();
  await page.getByRole('option', { name: '空闲', exact: true }).click();
  await expect(card.locator('.gpu-row')).toHaveCount(1);
  await expect(summary).toContainText('占用 1');
  await expect(summary).toContainText('150 W');
  await page.getByRole('combobox', { name: '状态筛选', exact: true }).click();
  await page.getByRole('option', { name: '全部状态', exact: true }).click();
  await card.getByRole('button', { name: '收起', exact: true }).click();
  await page.screenshot({ path: runtimePath('node-collapsed.png'), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(summary).toBeVisible();
  if (await page.evaluate(() => document.documentElement.scrollWidth > innerWidth))
    throw Error('Mobile overflow');
  await card.getByRole('button', { name: '展开', exact: true }).click();
  await card.getByRole('button', { name: '系统详情', exact: true }).click();
  await expect(card.locator('.node-expanded')).toBeVisible();
  await page.screenshot({ path: runtimePath('resources-mobile.png'), fullPage: true });
  if (await page.evaluate(() => document.documentElement.scrollWidth > innerWidth))
    throw Error('Expanded mobile overflow');
  if (errors.length) throw Error(errors.join('\n'));
  console.log(
    'Passed: whole-card collapse, footer details, refresh preserves state, summary totals, partial power, filter-independent totals, mobile.',
  );
} finally {
  await browser.close();
}
