import { expect, test } from '@playwright/test';
import {
	closeDetailSheet,
	detailPanel,
	detailSheet,
	expectDetailPanelScrollsToItsFooter,
	expectDetailSheetFitsTheViewport,
	expectMobileHeaderControlsShareOneRow,
	expectPersonDetailPanelContent,
	mockOrganizationDirectory
} from './organization-directory-helpers';

test.describe('employee organization directory mobile', () => {
	test('shows the selected person detail in a sheet that fits the viewport', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await mockOrganizationDirectory(page);

		await page.goto('/organization/');
		await page.getByTestId('organization-person-node-user-specimen-choi').click();

		await expect(detailSheet(page)).toBeVisible();
		await expectPersonDetailPanelContent(detailPanel(page));
		await expectDetailSheetFitsTheViewport(page);
		await closeDetailSheet(page);
		await expect(detailSheet(page)).toHaveCount(0);
	});

	test('keeps mobile controls compact and scrolls the edit panel to its footer', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 520 });
		await mockOrganizationDirectory(page, { canManage: true });

		await page.goto('/organization/');

		await expectMobileHeaderControlsShareOneRow(page);
		await page.getByTestId('organization-person-node-user-specimen-choi').click();

		const panel = detailPanel(page);
		await panel.getByRole('button', { name: '수정하기' }).click();
		await expect(page.getByTestId('organization-profile-user-specimen-choi')).toBeVisible();
		await expectDetailPanelScrollsToItsFooter(panel);
	});
});
