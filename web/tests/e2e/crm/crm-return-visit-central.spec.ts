import { expect, test } from '@playwright/test';
import { signInToTheCRM } from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

test('shows the CRM it already loaded when the user comes back to it', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 900 });
	await signInToTheCRM(page);

	await page.locator('a[href$="/task"]').first().click();
	await page.waitForURL(/\/task$/);
	await page.evaluate(() => {
		const observer = new MutationObserver(() => {
			if (document.querySelector('[data-testid="crm-loading-skeleton"]')) document.body.dataset.crmSkeletonSeen = 'true';
		});
		observer.observe(document.body, { childList: true, subtree: true });
	});

	await page.locator('a[href$="/crm"]').first().click();
	await page.waitForURL(/\/crm$/);
	await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
	await expect(page.locator('body')).not.toHaveAttribute('data-crm-skeleton-seen', 'true');
});
