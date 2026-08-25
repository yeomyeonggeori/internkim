import { expect, test } from '@playwright/test';
import {
	closeDetailSheet,
	detailPanel,
	expectDetailSheetBesideTheList,
	expectPersonDetailPanelContent,
	filterByPerson,
	mockOrganizationDirectory,
	selectOrganizationInTree,
	organizationDirectoryUsersResponse
} from './organization-directory-helpers';

test.describe('employee organization directory', () => {
	test('opens from the app shell and filters the hierarchical people layer', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 1000 });
		await mockOrganizationDirectory(page);

		await page.goto('/organization/');

		await expect(page.getByRole('link', { name: '조직' }).first()).toBeVisible();
		await expect(page.getByTestId('organization-board')).toBeVisible();
		await expect(page.getByTestId('organization-people-layer')).toBeVisible();
		await expect(page.getByTestId('organization-section-root')).toBeVisible();
		await expect(page.getByTestId('organization-section-product')).toBeVisible();
		await expect(page.getByTestId('organization-section-design')).toBeVisible();
		await expect(page.getByTestId('organization-person-node-user-example-jung')).toBeVisible();
		await expect(page.getByTestId('organization-person-node-user-sample-oh')).toBeVisible();
		await expect(page.getByRole('button', { name: '조직 작업' })).toHaveCount(0);
		await expect(detailPanel(page)).toHaveCount(0);
		await expect(page.getByText(/명 더 보기/)).toHaveCount(0);
		await expect(page.getByTestId('organization-person-node-user-specimen-choi').locator('img[alt="최견본"]')).toHaveAttribute('src', organizationDirectoryUsersResponse.records[2].image ?? '');

		await page.getByTestId('organization-person-node-user-specimen-choi').click();
		await expectPersonDetailPanelContent(detailPanel(page));
		await expectDetailSheetBesideTheList(page);
		await expect(detailPanel(page).getByRole('button', { name: '수정하기' })).toBeVisible();
		await closeDetailSheet(page);
		await expect(detailPanel(page)).toHaveCount(0);

		await page.getByTestId('organization-person-node-user-example-jung').click();
		await expect(detailPanel(page)).toContainText('정예시');
		await expect(detailPanel(page).getByRole('button', { name: '수정하기' })).toHaveCount(0);
		await closeDetailSheet(page);
		await expect(detailPanel(page)).toHaveCount(0);

		await filterByPerson(page, '프론트', '최견본');
		await expect(page.getByTestId('organization-person-node-user-sample-lee')).toHaveCount(0);
		await expect(page.getByTestId('organization-person-node-user-specimen-choi')).toBeVisible();

		await selectOrganizationInTree(page, 'product');
		await expect(page.getByTestId('organization-person-node-user-specimen-choi')).toBeVisible();
	});

	test('shows the empty state for an organization without members', async ({ page }) => {
		await mockOrganizationDirectory(page, {
			directoryResponse: {
				...organizationDirectoryUsersResponse,
				availableGroups: [...organizationDirectoryUsersResponse.availableGroups, { id: 'empty', name: '빈 조직' }]
			}
		});
		await page.goto('/organization/');

		await page.getByTestId('organization-row-empty').getByRole('button', { name: '빈 조직' }).click();

		await expect(page.getByRole('navigation', { name: 'breadcrumb' })).toContainText('빈 조직');
		await expect(page.getByText('표시할 조직도 구성원이 없습니다.')).toBeVisible();
	});

	test('shows company responsibility on the company leader inside an organization', async ({ page }) => {
		await mockOrganizationDirectory(page);
		await page.goto('/organization/');

		const leadershipSection = page.getByTestId('organization-section-leadership');
		const companyLeader = leadershipSection.getByTestId('organization-person-card-user-sample-lee');

		await expect(companyLeader.getByText('대표')).toBeVisible();
		await expect(companyLeader.getByText('책임자')).toHaveCount(0);
	});

	test('shows responsibility on the hierarchy leader instead of the earliest employee', async ({ page }) => {
		await mockOrganizationDirectory(page, {
			directoryResponse: {
				availableGroups: [
					{ id: 'leadership', name: '경영' },
					{ id: 'product', name: '제품팀' }
				],
				records: [
					{
						memberID: 'company-leader',
						handle: 'company-leader',
						name: '회사 리더',
						email: 'company-leader@example.com',
						hireDate: '2025-01-01',
						role: 'member',
						groupID: 'leadership'
					},
					{
						memberID: 'employee',
						handle: 'employee',
						name: '일반 직원',
						email: 'employee@example.com',
						hireDate: '2026-01-01',
						role: 'member',
						groupID: 'product',
						supervisorID: 'leader'
					},
					{
						memberID: 'leader',
						handle: 'leader',
						name: '조직 리더',
						email: 'leader@example.com',
						hireDate: '2026-02-01',
						role: 'member',
						groupID: 'product',
						supervisorID: 'company-leader'
					}
				]
			}
		});
		await page.goto('/organization/');

		const productSection = page.getByTestId('organization-section-product');
		await expect(productSection.getByTestId('organization-person-card-employee')).toBeVisible();
		await expect(productSection.getByTestId('organization-person-card-employee').getByText('책임자')).toHaveCount(0);
		await expect(productSection.getByTestId('organization-person-card-leader').getByText('책임자')).toBeVisible();
	});

	test('labels a person without an organization as unassigned', async ({ page }) => {
		await mockOrganizationDirectory(page);
		await page.goto('/organization/');

		await page.getByTestId('organization-person-node-user-sample-oh').click();

		await expect(detailPanel(page)).toContainText('팀 미지정');
	});

	test('localizes organization tree accessibility labels in English', async ({ page }) => {
		await mockOrganizationDirectory(page, { canManage: true, locale: 'en' });
		await page.goto('/organization/');

		await expect(page.getByTestId('organization-root')).toContainText('All');
		await expect(page.getByTestId('organization-root')).toContainText('3');
		await expect(page.getByRole('button', { name: '제품팀 Collapse' })).toBeVisible();

		await page.getByRole('button', { name: 'Organization actions' }).click();
		await page.getByRole('menuitem', { name: 'Edit', exact: true }).click();
		await expect(page.getByRole('button', { name: '제품팀 Move organization' })).toHaveCount(0);
		const dragHandle = page.getByTestId('organization-drag-handle-product');
		await expect(dragHandle).toHaveAttribute('aria-hidden', 'true');
		expect(await dragHandle.evaluate((element) => element.tabIndex)).toBe(-1);
	});
});
