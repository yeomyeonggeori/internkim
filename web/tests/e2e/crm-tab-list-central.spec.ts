import { expect, test } from '@playwright/test';
import { signInToTheCRM } from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

test('keeps the primary CRM tab list static without horizontal scrolling', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await signInToTheCRM(page);
	const tabList = page.locator('[data-slot="underline-tabs-list"]').first();
	await expect(tabList).toHaveCSS('overflow-x', 'clip');
	const overflow = await tabList.evaluate((element) => element.scrollWidth - element.clientWidth);
	expect(overflow).toBeLessThanOrEqual(1);
});
