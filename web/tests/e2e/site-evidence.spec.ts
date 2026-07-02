import { expect, test } from '@playwright/test';

const publicURL = process.env.INTERNKIM_PUBLIC_URL ?? '';
const screenshotPath = process.env.INTERNKIM_SITE_SCREENSHOT ?? '';

test('capture public website evidence', async ({ page }) => {
	test.setTimeout(120000);
	expect(publicURL).not.toEqual('');
	expect(screenshotPath).not.toEqual('');

	const response = await page.goto(publicURL, { waitUntil: 'domcontentloaded', timeout: 60000 });
	expect(response?.status() ?? 0).toBeLessThan(500);

	await page.waitForLoadState('networkidle', { timeout: 15000 }).catch(() => {});
	const body = page.locator('body');
	await expect(body).not.toContainText(/Bad gateway|Sorry, we could not find the page|INTERNKIM_SITE_STARTER_REPLACE_ME|Replace this starter/i);
	await page.screenshot({ path: screenshotPath, fullPage: true });
});
