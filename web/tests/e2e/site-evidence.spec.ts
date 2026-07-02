import { expect, test } from '@playwright/test';

const publicURL = process.env.INTERNKIM_PUBLIC_URL ?? '';
const screenshotPath = process.env.INTERNKIM_SITE_SCREENSHOT ?? '';
const evidenceMode = process.env.INTERNKIM_SITE_EVIDENCE_MODE ?? 'published';
const expectedText = process.env.INTERNKIM_SITE_EXPECT_TEXT ?? '';
const absentText = process.env.INTERNKIM_SITE_ABSENT_TEXT ?? '';

test('capture public website evidence', async ({ page }) => {
	test.setTimeout(120000);
	expect(publicURL).not.toEqual('');
	expect(screenshotPath).not.toEqual('');

	const response = await page.goto(publicURL, { waitUntil: 'domcontentloaded', timeout: 60000 });
	await page.waitForLoadState('networkidle', { timeout: 15000 }).catch(() => {});
	const body = page.locator('body');
	const statusCode = response?.status() ?? 0;

	expect(statusCode).toBeLessThan(500);
	if (evidenceMode === 'deleted') {
		if (absentText !== '') await expect(body).not.toContainText(absentText);
		const bodyText = await body.innerText({ timeout: 5000 }).catch(() => '');
		expect(statusCode >= 400 || /not found|could not find|deleted|삭제/i.test(bodyText)).toBeTruthy();
		await page.screenshot({ path: screenshotPath, fullPage: true });
		return;
	}

	expect(statusCode).toBeGreaterThanOrEqual(200);
	expect(statusCode).toBeLessThan(400);
	await expect(body).not.toContainText(/Bad gateway|Sorry, we could not find the page|INTERNKIM_SITE_STARTER_REPLACE_ME|Replace this starter/i);
	if (expectedText !== '') await expect(body).toContainText(expectedText, { timeout: 30000 });
	await page.screenshot({ path: screenshotPath, fullPage: true });
});
