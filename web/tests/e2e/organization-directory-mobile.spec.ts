import { expect, test } from '@playwright/test';
import {
	expectMobileDetailPanelScrollsToBottom,
	expectMobileDetailSheetLayout,
	expectMobileHeaderControlsInTitleRow,
	expectPersonDetailPanelContent,
	mockOrganizationDirectory
} from './organization-directory-helpers';

test.describe('employee organization directory mobile', () => {
	test('shows the selected person detail in a mobile bottom sheet', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await mockOrganizationDirectory(page);

		await page.goto('/organization/');
		await page.getByTestId('organization-person-node-user-dabin').click();

		const sheet = page.getByTestId('organization-mobile-detail-sheet');
		const detailPanel = sheet.getByTestId('organization-person-detail-panel');
		await expect(page.getByTestId('organization-detail-column')).toHaveCount(0);
		await expect(sheet).toBeVisible();
		await expectPersonDetailPanelContent(detailPanel);
		await expectMobileDetailSheetLayout(page);
		await detailPanel.getByRole('button', { name: '상세 닫기' }).click();
		await expect(page.getByTestId('organization-mobile-detail-sheet')).toHaveCount(0);
	});

	test('keeps mobile controls compact and scrolls the edit sheet to the bottom', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 520 });
		await mockOrganizationDirectory(page, { canManage: true });

		await page.goto('/organization/');

		await expectMobileHeaderControlsInTitleRow(page);
		await page.getByTestId('organization-person-node-user-dabin').click();

		const detailPanel = page.getByTestId('organization-mobile-detail-sheet').getByTestId('organization-person-detail-panel');
		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await expect(detailPanel.getByRole('heading', { name: '편집' })).toBeVisible();
		await expectMobileDetailPanelScrollsToBottom(detailPanel);
	});
});
