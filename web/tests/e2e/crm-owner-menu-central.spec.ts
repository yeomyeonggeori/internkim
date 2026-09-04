import { expect, test } from '@playwright/test';
import { openQuickAdd, recordSheet, signInToTheCRM } from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

test('keeps the searchable internal owner menu aligned to its trigger', async ({ page }) => {
	await signInToTheCRM(page);
	await openQuickAdd(page, '관계처');
	const sheet = recordSheet(page);
	const trigger = sheet.getByLabel('내부 담당자');
	await trigger.click();
	const menu = page.locator('[data-slot="popover-content"]:visible');
	await expect(menu).toBeVisible();
	await expect
		.poll(async () => {
			const triggerWidth = await trigger.evaluate((element) => element.getBoundingClientRect().width);
			const menuWidth = await menu.evaluate((element) => element.getBoundingClientRect().width);
			return Math.abs(menuWidth - triggerWidth);
		})
		.toBeLessThanOrEqual(2);
});
