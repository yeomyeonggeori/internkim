import { expect, test } from '@playwright/test';
import {
	expectMobileDetailPanelScrollsToBottom,
	expectMobileDetailSheetLayout,
	expectMobileHeaderControlsInTitleRow,
	expectPersonDetailPanelContent,
	mockOrgchartDirectory
} from './orgchart-directory-helpers';

test.describe('employee orgchart directory mobile', () => {
	test('shows the selected person detail in a mobile bottom sheet', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await mockOrgchartDirectory(page);

		await page.goto('/orgchart/');
		await page.getByTestId('orgchart-person-node-user-dabin').click();

		const sheet = page.getByTestId('orgchart-mobile-detail-sheet');
		const detailPanel = sheet.getByTestId('orgchart-person-detail-panel');
		await expect(page.getByTestId('orgchart-detail-column')).toHaveCount(0);
		await expect(sheet).toBeVisible();
		await expectPersonDetailPanelContent(detailPanel);
		await expectMobileDetailSheetLayout(page);
		await detailPanel.getByRole('button', { name: '상세 닫기' }).click();
		await expect(page.getByTestId('orgchart-mobile-detail-sheet')).toHaveCount(0);
	});

	test('keeps mobile controls compact and scrolls the edit sheet to the bottom', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 520 });
		await mockOrgchartDirectory(page, { canManage: true });

		await page.goto('/orgchart/');

		await expectMobileHeaderControlsInTitleRow(page);
		await page.getByTestId('orgchart-person-node-user-dabin').click();

		const detailPanel = page.getByTestId('orgchart-mobile-detail-sheet').getByTestId('orgchart-person-detail-panel');
		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await expect(detailPanel.getByRole('heading', { name: '편집' })).toBeVisible();
		await expectMobileDetailPanelScrollsToBottom(detailPanel);
	});
});
