import { expect, test } from '@playwright/test';
import { openCreateForm, recordSheet, signInToTheCRM } from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

const organizationName = '예시 협력 기관';
const contactName = '최견본';

test('keeps a picker popover above the sheet that opened it', async ({ page }) => {
	await signInToTheCRM(page);
	await openCreateForm(page, '거래', '거래');
	const sheet = recordSheet(page);
	await sheet.getByLabel('관계처').click();
	await page.getByRole('option', { name: organizationName, exact: true }).click();
	await sheet.locator('#crm-record-progress-contact').click();
	await expect(page.getByRole('option', { name: new RegExp(contactName) })).toBeVisible();

	const topmost = await page.evaluate(() => {
		const popover = document.querySelector('[data-slot="popover-content"]');
		if (!(popover instanceof HTMLElement)) return 'popover missing';
		const box = popover.getBoundingClientRect();
		const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
		if (!hit) return 'nothing at the popover centre';
		if (popover.contains(hit)) return 'popover';
		const sheetContent = hit.closest('[data-slot="sheet-content"]');
		return sheetContent ? 'sheet' : 'other';
	});

	expect(topmost).toBe('popover');
});
