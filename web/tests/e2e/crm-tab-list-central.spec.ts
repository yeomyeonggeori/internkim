import { expect, test } from '@playwright/test';
import { signInToTheCRM } from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

test('keeps every primary CRM tab reachable with a full touch target', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await signInToTheCRM(page);
	const tabList = page.locator('[data-slot="underline-tabs-list"]').first();
	await expect(tabList).toHaveCSS('overflow-x', 'auto');
	for (const tab of await tabList.getByRole('tab').all()) {
		await tab.scrollIntoViewIfNeeded();
		const bounds = await tab.boundingBox();
		expect(bounds?.height).toBeGreaterThanOrEqual(44);
		await tab.click();
		await expect(tab).toHaveAttribute('aria-selected', 'true');
	}
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
