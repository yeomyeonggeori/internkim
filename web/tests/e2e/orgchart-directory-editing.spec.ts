import { expect, test } from '@playwright/test';
import {
	expectDetailPanelInRightColumn,
	expectDetailPanelStableWhileListScrolls,
	expectPersonDetailPanelContent,
	mockOrgchartDirectory,
	openFilterPopover
} from './orgchart-directory-helpers';

test.describe('employee orgchart directory editing', () => {
	test('keeps the detail panel fixed while admins edit the selected person', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 420 });
		await mockOrgchartDirectory(page, { canManage: true });

		await page.goto('/orgchart/');

		await expect(page.getByRole('button', { name: '조직 추가' })).toBeVisible();
		await expect(page.getByRole('button', { name: '편집' })).toHaveCount(0);
		await expect(page.getByTestId('orgchart-person-edit-user-dabin')).toHaveCount(0);
		await expect(page.getByTestId('orgchart-team-column-product')).toBeVisible();

		await page.getByTestId('orgchart-person-node-user-dabin').click();
		const detailPanel = page.getByTestId('orgchart-person-detail-panel');
		await expectPersonDetailPanelContent(detailPanel);
		await expect(detailPanel.getByRole('button', { name: '수정하기' })).toBeVisible();
		await expectDetailPanelInRightColumn(page);
		await expectDetailPanelStableWhileListScrolls(page);

		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await expect(detailPanel.getByRole('heading', { name: '편집' })).toBeVisible();
		await expect(page.getByTestId('orgchart-profile-user-dabin').getByLabel('직책', { exact: true })).toBeVisible();
	});

	test('keeps an unsaved edit draft when changing the organization filter', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 800 });
		await mockOrgchartDirectory(page, { canManage: true });

		await page.goto('/orgchart/');
		await page.getByTestId('orgchart-person-node-user-dabin').click();

		const detailPanel = page.getByTestId('orgchart-person-detail-panel');
		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await detailPanel.getByLabel('직책', { exact: true }).fill('저장 전 직책');
		await openFilterPopover(page);
		await page.getByRole('button', { name: '조직', exact: true }).click();
		await page.getByRole('option', { name: '디자인팀' }).click();

		await expect(page.getByTestId('orgchart-filter-popover')).toHaveCount(0);
		await expect(page.getByText('저장하지 않은 조직도 변경사항이 있습니다.')).toBeVisible();
		await expect(detailPanel.getByLabel('직책', { exact: true })).toHaveValue('저장 전 직책');
		await expect(page.getByTestId('orgchart-person-node-user-dabin')).toBeVisible();
	});

	test('keeps an unsaved edit draft when closing the detail panel', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 800 });
		await mockOrgchartDirectory(page, { canManage: true });

		await page.goto('/orgchart/');
		await page.getByTestId('orgchart-person-node-user-dabin').click();

		const detailPanel = page.getByTestId('orgchart-person-detail-panel');
		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await detailPanel.getByLabel('직책', { exact: true }).fill('닫기 전 직책');
		await detailPanel.getByRole('button', { name: '상세 닫기' }).click();

		await expect(page.getByText('저장하지 않은 조직도 변경사항이 있습니다.')).toBeVisible();
		await expect(detailPanel).toBeVisible();
		await expect(detailPanel.getByLabel('직책', { exact: true })).toHaveValue('닫기 전 직책');

		await detailPanel.getByRole('button', { name: '취소' }).click();
		await detailPanel.getByRole('button', { name: '상세 닫기' }).click();
		await expect(page.getByTestId('orgchart-person-detail-panel')).toHaveCount(0);
	});

	test('keeps an unsaved edit draft when search hides the selected person from the list', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 800 });
		await mockOrgchartDirectory(page, { canManage: true });

		await page.goto('/orgchart/');
		await page.getByTestId('orgchart-person-node-user-dabin').click();

		const detailPanel = page.getByTestId('orgchart-person-detail-panel');
		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await detailPanel.getByLabel('직책', { exact: true }).fill('검색 중 직책');
		await page.getByLabel('검색').fill('박지은');

		await expect(page.getByTestId('orgchart-person-node-user-dabin')).toHaveCount(0);
		await expect(detailPanel).toBeVisible();
		await expect(detailPanel).toContainText('김다빈');
		await expect(detailPanel.getByLabel('직책', { exact: true })).toHaveValue('검색 중 직책');
		await expect(detailPanel.getByRole('button', { name: '저장' })).toBeVisible();
		await expect(detailPanel.getByRole('button', { name: '취소' })).toBeVisible();
	});
});
