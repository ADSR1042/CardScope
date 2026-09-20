import { runtimePath } from './paths.mjs';
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs';
const access = JSON.parse(fs.readFileSync(runtimePath('access.json'), 'utf8'));
const browser = await chromium.launch({ channel: 'msedge', headless: true });
try {
  const page = await browser.newPage();
  await page.goto(access.url);
  await page.getByLabel('账号', { exact: true }).fill('viewer');
  await page.getByLabel('密码', { exact: true }).fill(access.viewer);
  await page.getByRole('button', { name: '进入面板', exact: true }).click();
  const card = page.locator('.node-card').first(),
    header = card.locator('.node-header'),
    body = card.locator('.gpu-table');
  await expect(header).toHaveAttribute('aria-expanded', 'false');
  await expect(body).toBeHidden();
  await header.locator('.node-identity').click();
  await expect(body).toBeVisible();
  await header.locator('.node-power').click();
  await expect(body).toBeHidden();
  await header.focus();
  await page.keyboard.press('Enter');
  await expect(body).toBeVisible();
  await page.keyboard.press('Space');
  await expect(body).toBeHidden();
  const bounds = () => page.locator('.page').boundingBox();
  const before = await bounds();
  await page.evaluate(() => {
    const spacer = document.createElement('div');
    spacer.id = 'scroll-check';
    spacer.style.height = '200vh';
    document.body.append(spacer);
  });
  const after = await bounds();
  if (before.x !== after.x || before.width !== after.width) throw Error('Scrollbar shifted page');
  await page.evaluate(() => document.querySelector('#scroll-check').remove());
  console.log('Passed: page width and horizontal position stable across scrollbar changes.');
} finally {
  await browser.close();
}
