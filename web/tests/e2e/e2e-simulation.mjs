import { chromium } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { root } from './paths.mjs';
const access = JSON.parse(
  fs.readFileSync(path.join(root, '.runtime/simulation-access.json'), 'utf8'),
);
const browser = await chromium.launch({ channel: 'msedge', headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } });
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));
await page.goto(access.url);
await page.getByLabel('账号', { exact: true }).fill('admin');
await page.getByLabel('密码', { exact: true }).fill(access.admin);
const start = Date.now();
await page.getByRole('button', { name: '进入工作空间' }).click();
await page.locator('.gpu-row').nth(159).waitFor();
const loadMs = Date.now() - start;
if ((await page.locator('.node-card').count()) !== 20) throw new Error('Expected 20 nodes');
await page.screenshot({ path: path.join(root, '.runtime/simulation-160-gpus.png') });
await page.getByRole('combobox', { name: '状态筛选', exact: true }).click();
await page.getByRole('option', { name: '采集或健康异常', exact: true }).click();
if ((await page.locator('.gpu-row').count()) !== 1) throw new Error('Fault filtering failed');
await page.locator('.gpu-row').click();
await page.getByRole('dialog').waitFor();
await page.screenshot({ path: path.join(root, '.runtime/simulation-fault.png') });
await page.getByRole('button', { name: '关闭', exact: true }).click();
await page.getByRole('combobox', { name: '状态筛选', exact: true }).click();
await page.getByRole('option', { name: '全部状态', exact: true }).click();
await page.getByLabel('搜索节点或用户').fill('alice');
if ((await page.locator('.gpu-row').count()) !== 30) throw new Error('User filter failed');
await page.getByLabel('搜索节点或用户').fill('');
await page.getByRole('button', { name: '使用统计', exact: true }).click();
await page.getByText('用户持卡时长', { exact: true }).waitFor();
await page.locator('tbody tr').nth(159).waitFor();
await page.screenshot({ path: path.join(root, '.runtime/simulation-statistics.png') });
if (errors.length) throw new Error(errors.join(';'));
fs.writeFileSync(
  path.join(root, '.runtime/simulation-browser-result.json'),
  JSON.stringify(
    {
      passed: true,
      nodes: 20,
      gpus: 160,
      loadMs,
      checks: [
        '160 GPU rows',
        '20 server cards',
        'fault filter and drawer',
        'user filter',
        'user statistics',
      ],
      pageErrors: errors,
    },
    null,
    2,
  ),
);
await browser.close();
console.log(`Simulation UI passed: 20 nodes, 160 GPUs, load ${loadMs} ms.`);
