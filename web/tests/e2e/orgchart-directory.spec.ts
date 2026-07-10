import { expect, test } from '@playwright/test';
import {
	expectDetailPanelInRightColumn,
	expectPersonDetailPanelContent,
	mockOrgchartDirectory,
	openFilterPopover,
	orgchartDirectoryUsersResponse
} from './orgchart-directory-helpers';

test.describe('employee orgchart directory', () => {
	test('opens from the app shell and filters read-only organization cards', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 1000 });
		await mockOrgchartDirectory(page);

		await page.goto('/orgchart/');

		await expect(page.getByRole('link', { name: '조직도' }).first()).toBeVisible();
		await expect(page.getByRole('heading', { name: '직원' })).toBeVisible();
		await expect(page.getByTestId('orgchart-board')).toBeVisible();
		await expect(page.getByTestId('orgchart-organization-grid')).toBeVisible();
		await expect(page.getByTestId('orgchart-team-column-product')).toBeVisible();
		await expect(page.getByTestId('orgchart-team-column-__unassigned__')).toBeVisible();
		await expect(page.getByTestId('orgchart-tree-node-user-taehyun')).toBeVisible();
		await expect(page.getByRole('button', { name: '편집' })).toHaveCount(0);
		await expect(page.getByTestId('orgchart-person-detail-panel')).toHaveCount(0);
		await expect(page.getByTestId('orgchart-team-column-product').getByText('팀장')).toHaveCount(1);
		await expect(page.getByTestId('orgchart-team-column-product').getByText('active')).toHaveCount(0);
		await expect(page.getByTestId('orgchart-person-node-user-dabin').locator('img[alt="김다빈"]')).toHaveAttribute('src', orgchartDirectoryUsersResponse.records[2].image ?? '');

		await page.getByTestId('orgchart-person-node-user-dabin').click();
		await expectPersonDetailPanelContent(page.getByTestId('orgchart-person-detail-panel'));
		await expectDetailPanelInRightColumn(page);
		await expect(page.getByTestId('orgchart-person-detail-panel').getByRole('button', { name: '수정하기' })).toHaveCount(0);
		await page.getByRole('button', { name: '상세 닫기' }).click();
		await expect(page.getByTestId('orgchart-person-detail-panel')).toHaveCount(0);

		await page.getByLabel('검색').fill('없는직원');
		await expect(page.getByText('표시할 조직도 구성원이 없습니다.')).toBeVisible();
		await expect(page.getByTestId('orgchart-person-node-user-dabin')).toHaveCount(0);

		await page.getByLabel('검색').fill('프론트');
		await expect(page.getByTestId('orgchart-person-node-user-ceo')).toHaveCount(0);
		await expect(page.getByTestId('orgchart-person-node-user-dabin')).toBeVisible();

		await openFilterPopover(page);
		await page.getByRole('button', { name: '조직', exact: true }).click();
		await page.getByRole('option', { name: '제품팀' }).click();
		await expect(page.getByTestId('orgchart-filter-popover')).toHaveCount(0);
		await expect(page.getByTestId('orgchart-person-node-user-dabin')).toBeVisible();
	});
});
