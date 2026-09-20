import { runtimePath } from './paths.mjs';
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
const access = JSON.parse(fs.readFileSync(runtimePath('access.json'), 'utf8'));
const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
page.setDefaultTimeout(10000);
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));
try {
  await page.goto(access.url);
  await page.getByLabel('账号', { exact: true }).fill('viewer');
  await page.getByLabel('密码', { exact: true }).fill(access.viewer);
  await page.getByRole('button', { name: '进入面板', exact: true }).click();
  await page.getByRole('button', { name: '使用统计', exact: true }).click();
  await expect(page.getByRole('heading', { name: '使用统计', exact: true })).toBeVisible();
  await expect(page.locator('.user-stats-row')).toHaveCount(3);
  await expect(page.getByText('整卡使用', { exact: true })).toHaveCount(0);
  const alice = page.locator('.user-stats-row').filter({ hasText: 'demo-alice' });
  await expect(alice.locator('td').last()).toHaveText('2');
  await alice.getByRole('button').click();
  await expect(page.locator('.user-stats-detail tbody tr')).toHaveCount(2);
  await page.screenshot({ path: runtimePath('user-statistics.png'), fullPage: true });
  await page.getByLabel('统计用户').fill('demo-bob');
  await expect(page.locator('.user-stats-row')).toHaveCount(1);
  const downloadPromise = page.waitForEvent('download');
  await page.getByRole('button', { name: '导出', exact: true }).click();
  const download = await downloadPromise;
  await download.saveAs(runtimePath('user-statistics.csv'));
  const csv = fs.readFileSync(runtimePath('user-statistics.csv'), 'utf8');
  if (!csv.includes('demo-bob') || csv.includes('demo-alice') || csv.includes('weighted_hours'))
    throw Error('Export filter mismatch');
  await page.getByLabel('统计用户').fill('no-such-user');
  await expect(page.getByText('没有匹配的使用记录', { exact: true })).toBeVisible();
  await page.getByLabel('统计用户').fill('');
  await page.getByRole('combobox', { name: '统计范围', exact: true }).click();
  await page.getByRole('option', { name: '自定义时间', exact: true }).click();
  await page.getByLabel('开始时间', { exact: true }).fill('2026-09-18T15:00');
  await page.getByLabel('结束时间', { exact: true }).fill('2026-09-17T15:00');
  await expect(page.getByRole('alert')).toHaveText('结束时间须晚于开始时间');
  await page.getByRole('combobox', { name: '统计范围', exact: true }).click();
  await page.getByRole('option', { name: '最近 7 天', exact: true }).click();
  await expect(page.locator('.user-stats-row')).toHaveCount(3);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: runtimePath('user-statistics-mobile.png'), fullPage: true });
  if (await page.evaluate(() => document.documentElement.scrollWidth > innerWidth))
    throw Error('Mobile overflow');
  if (errors.length) throw Error(errors.join('\n'));
  console.log(
    'Passed: user grouping, two-card details, filter/export, empty state, invalid dates, mobile layout; no browser errors.',
  );
} finally {
  await browser.close();
}
