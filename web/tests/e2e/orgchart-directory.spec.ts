import { expect, test } from '@playwright/test';
import {
	expectDetailPanelInRightColumn,
	expectPersonDetailPanelContent,
	mockOrgchartDirectory,
	openFilterPopover,
	orgchartDirectoryUsersResponse
} from './orgchart-directory-helpers';

test.describe('employee orgchart directory', () => {
	test('opens from the app shell and filters the hierarchical people layer', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 1000 });
		await mockOrgchartDirectory(page);

		await page.goto('/orgchart/');

		await expect(page.getByRole('link', { name: '조직도' }).first()).toBeVisible();
		await expect(page.getByRole('heading', { name: '조직도' })).toBeVisible();
		await expect(page.getByTestId('orgchart-board')).toBeVisible();
		await expect(page.getByTestId('orgchart-people-layer')).toBeVisible();
		await expect(page.getByTestId('orgchart-organization-section-root')).toBeVisible();
		await expect(page.getByTestId('orgchart-organization-section-product')).toBeVisible();
		await expect(page.getByTestId('orgchart-organization-section-design')).toBeVisible();
		await expect(page.getByTestId('orgchart-person-node-user-taehyun')).toBeVisible();
		await expect(page.getByTestId('orgchart-person-node-user-nam')).toBeVisible();
		await expect(page.getByRole('button', { name: '편집' })).toHaveCount(0);
		await expect(page.getByTestId('orgchart-person-detail-panel')).toHaveCount(0);
		await expect(page.getByText(/명 더 보기/)).toHaveCount(0);
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

	test('shows the empty state for an organization without members', async ({ page }) => {
		await mockOrgchartDirectory(page, {
			directoryResponse: {
				...orgchartDirectoryUsersResponse,
				availableGroups: [...orgchartDirectoryUsersResponse.availableGroups, { id: 'empty', name: '빈 조직' }]
			}
		});
		await page.goto('/orgchart/');

		await page.getByTestId('orgchart-organization-row-empty').getByRole('button', { name: '빈 조직' }).click();

		await expect(page.getByRole('heading', { name: '빈 조직' })).toBeVisible();
		await expect(page.getByText('표시할 조직도 구성원이 없습니다.')).toBeVisible();
	});

	test('shows company responsibility on the company leader inside an organization', async ({ page }) => {
		await mockOrgchartDirectory(page);
		await page.goto('/orgchart/');

		const leadershipSection = page.getByTestId('orgchart-organization-section-leadership');
		const companyLeader = leadershipSection.getByTestId('orgchart-person-node-user-ceo');

		await expect(companyLeader.getByText('회사 책임자')).toBeVisible();
		await expect(companyLeader.getByText('조직 책임자')).toHaveCount(0);
	});

	test('shows responsibility on the hierarchy leader instead of the earliest employee', async ({ page }) => {
		await mockOrgchartDirectory(page, {
			directoryResponse: {
				availableGroups: [
					{ id: 'leadership', name: '경영' },
					{ id: 'product', name: '제품팀' }
				],
				records: [
					{
						userID: 'company-leader',
						handle: 'company-leader',
						name: '회사 리더',
						email: 'company-leader@example.com',
						hireDate: '2025-01-01',
						role: 'member',
						primaryGroupID: 'leadership',
						groupIDs: ['leadership']
					},
					{
						userID: 'employee',
						handle: 'employee',
						name: '일반 직원',
						email: 'employee@example.com',
						hireDate: '2026-01-01',
						role: 'member',
						primaryGroupID: 'product',
						groupIDs: ['product'],
						supervisorID: 'leader'
					},
					{
						userID: 'leader',
						handle: 'leader',
						name: '조직 리더',
						email: 'leader@example.com',
						hireDate: '2026-02-01',
						role: 'member',
						primaryGroupID: 'product',
						groupIDs: ['product'],
						supervisorID: 'company-leader'
					}
				]
			}
		});
		await page.goto('/orgchart/');

		const productSection = page.getByTestId('orgchart-organization-section-product');
		await expect(productSection.getByTestId('orgchart-person-node-employee')).toBeVisible();
		await expect(productSection.getByTestId('orgchart-person-node-employee').getByText('조직 책임자')).toHaveCount(0);
		await expect(productSection.getByTestId('orgchart-person-node-leader').getByText('조직 책임자')).toBeVisible();
	});

	test('labels the unassigned organization consistently', async ({ page }) => {
		await mockOrgchartDirectory(page);
		await page.goto('/orgchart/');

		await openFilterPopover(page);
		await page.getByRole('button', { name: '조직', exact: true }).click();
		await page.getByRole('option', { name: '팀 미지정' }).click();

		await expect(page.getByRole('heading', { name: '팀 미지정' })).toBeVisible();
		await expect(page.getByTestId('orgchart-organization-section-root')).toContainText('팀 미지정');
		await expect(page.getByTestId('orgchart-organization-section-root')).toContainText('1명');
		await expect(page.getByTestId('orgchart-person-node-user-nam')).toBeVisible();
		await expect(page.getByTestId('orgchart-person-node-user-dabin')).toHaveCount(0);
	});

	test('localizes organization tree accessibility labels in English', async ({ page }) => {
		await mockOrgchartDirectory(page, { canManage: true, locale: 'en' });
		await page.goto('/orgchart/');

		await expect(page.getByTestId('orgchart-organization-root').getByTestId('orgchart-avatar-stack')).toHaveAttribute('aria-label', '7 people');
		await expect(page.getByRole('button', { name: '제품팀 Collapse' })).toBeVisible();

		await page.getByRole('button', { name: 'Edit', exact: true }).click();
		await expect(page.getByRole('button', { name: '제품팀 Move organization' })).toHaveCount(0);
		const dragHandle = page.getByTestId('orgchart-organization-drag-handle-product');
		await expect(dragHandle).toHaveAttribute('aria-hidden', 'true');
		expect(await dragHandle.evaluate((element) => element.tabIndex)).toBe(-1);
	});
});
