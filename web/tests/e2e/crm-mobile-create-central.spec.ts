import { expect, test } from '@playwright/test';
import {
	expectNoHorizontalOverflow,
	openQuickAdd,
	recordSheet,
	removeOrganizationsNamed,
	signInToTheCRM
} from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

const organizationName = 'E2E 모바일 관계처';

test.afterEach(async () => {
	await removeOrganizationsNamed([organizationName]);
});

test('completes a create flow on a mobile viewport', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await signInToTheCRM(page);
	await expectNoHorizontalOverflow(page.locator('html'));

	await openQuickAdd(page, '관계처');
	const sheet = recordSheet(page);
	await expectNoHorizontalOverflow(sheet);
	await sheet.getByLabel('이름 또는 제목').fill(organizationName);
	await sheet.getByRole('button', { name: '추가', exact: true }).click();

	await expect(sheet).not.toBeVisible();
	await expect(page.getByRole('row', { name: new RegExp(organizationName) })).toBeVisible();
});
