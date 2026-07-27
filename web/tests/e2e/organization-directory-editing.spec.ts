import { expect, test } from '@playwright/test';
import {
	expectDetailPanelInRightColumn,
	expectDetailPanelStableWhileListScrolls,
	expectPersonDetailPanelContent,
	mockOrganizationDirectory,
	selectOrganizationInTree
} from './organization-directory-helpers';

type SavedGroup = { id: string; name: string; parentID?: string };

test.describe('employee organization directory editing', () => {
	test('adds an organization under the selected parent', async ({ page }) => {
		let savedGroups: SavedGroup[] = [];
		await mockOrganizationDirectory(page, { canManage: true });
		await page.route('**/admin/api/org-groups**', async (route) => {
			const payload = route.request().postDataJSON() as { groups: SavedGroup[] };
			savedGroups = payload.groups;
			await route.fulfill({ json: { availableGroups: payload.groups, records: [] } });
		});
		await page.goto('/organization/');

		await page.getByRole('button', { name: '조직 추가' }).click();
		await page.getByLabel('새 조직').fill('개발팀');
		await page.getByRole('button', { name: '상위 조직' }).click();
		await page.getByRole('option', { name: '제품팀' }).click();
		await page.getByRole('button', { name: '추가', exact: true }).click();

		await expect.poll(() => savedGroups.find((group) => group.name === '개발팀')?.parentID).toBe('product');
	});

	test('uses horizontal drag position to move a child organization to the top level', async ({ page }) => {
		let savedGroups: SavedGroup[] = [];
		await mockOrganizationDirectory(page, { canManage: true });
		await page.route('**/admin/api/org-groups**', async (route) => {
			const payload = route.request().postDataJSON() as { groups: SavedGroup[] };
			savedGroups = payload.groups;
			await route.fulfill({ json: { availableGroups: payload.groups, records: [] } });
		});
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto('/organization/');
		await page.getByRole('button', { name: '편집', exact: true }).click();

		const sidebar = page.getByTestId('organization-sidebar');
		const handle = page.getByTestId('organization-drag-handle-design');
		const fieldRow = page.getByTestId('organization-row-field');
		const sidebarBox = await sidebar.boundingBox();
		const handleBox = await handle.boundingBox();
		const fieldBox = await fieldRow.boundingBox();
		if (!sidebarBox || !handleBox || !fieldBox) throw new Error('organization drag geometry unavailable');
		await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2);
		await page.mouse.down();
		await page.mouse.move(sidebarBox.x + 28, fieldBox.y + fieldBox.height + 8, { steps: 8 });

		await expect(page.getByTestId('organization-drop-preview')).toContainText('회사 바로 아래');
		await page.mouse.up();
		await page.getByRole('button', { name: '저장', exact: true }).click();

		await expect.poll(() => savedGroups.find((group) => group.id === 'design')?.parentID ?? '').toBe('');
		await expect(page.getByText('조직 구조를 편집하고 있습니다.')).toHaveCount(0);
	});

	test('keeps a top-level drop outside the preceding organization subtree', async ({ page }) => {
		let savedGroups: SavedGroup[] = [];
		await mockOrganizationDirectory(page, { canManage: true });
		await page.route('**/admin/api/org-groups**', async (route) => {
			const payload = route.request().postDataJSON() as { groups: SavedGroup[] };
			savedGroups = payload.groups;
			await route.fulfill({ json: { availableGroups: payload.groups, records: [] } });
		});
		await page.setViewportSize({ width: 1440, height: 900 });
		await page.goto('/organization/');
		await page.getByRole('button', { name: '편집', exact: true }).click();

		const sidebar = page.getByTestId('organization-sidebar');
		const handle = page.getByTestId('organization-drag-handle-field');
		const designRow = page.getByTestId('organization-row-design');
		const sidebarBox = await sidebar.boundingBox();
		const handleBox = await handle.boundingBox();
		const designBox = await designRow.boundingBox();
		if (!sidebarBox || !handleBox || !designBox) throw new Error('organization hierarchy drag geometry unavailable');
		await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2);
		await page.mouse.down();
		await page.mouse.move(sidebarBox.x + 28, designBox.y + 1, { steps: 8 });

		const previewBox = await page.getByTestId('organization-drop-preview').boundingBox();
		if (!previewBox) throw new Error('organization hierarchy drop preview unavailable');
		expect(previewBox.y).toBeGreaterThan(designBox.y + designBox.height);
		await page.mouse.up();
		await page.getByRole('button', { name: '저장', exact: true }).click();

		await expect.poll(() => savedGroups.map((group) => group.id)).toEqual(['leadership', 'product', 'design', 'field']);
	});

	test('keeps the detail panel fixed while admins edit the selected person', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 420 });
		await mockOrganizationDirectory(page, { canManage: true });

		await page.goto('/organization/');

		await expect(page.getByRole('button', { name: '조직 추가' })).toBeVisible();
		await expect(page.getByRole('button', { name: '편집', exact: true })).toBeVisible();
		await expect(page.getByTestId('organization-person-edit-user-dabin')).toHaveCount(0);
		await expect(page.getByTestId('organization-section-product')).toBeVisible();

		await page.getByTestId('organization-person-node-user-dabin').click();
		const detailPanel = page.getByTestId('organization-person-detail-panel');
		await expectPersonDetailPanelContent(detailPanel);
		await expect(detailPanel.getByRole('button', { name: '수정하기' })).toBeVisible();
		await expectDetailPanelInRightColumn(page);
		await expectDetailPanelStableWhileListScrolls(page);

		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await expect(detailPanel.getByRole('heading', { name: '편집' })).toBeVisible();
		await expect(page.getByTestId('organization-profile-user-dabin').getByLabel('직책', { exact: true })).toBeVisible();
	});

	test('keeps an unsaved edit draft when changing the organization filter', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 800 });
		await mockOrganizationDirectory(page, { canManage: true });

		await page.goto('/organization/');
		await page.getByTestId('organization-person-node-user-dabin').click();

		const detailPanel = page.getByTestId('organization-person-detail-panel');
		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await detailPanel.getByLabel('직책', { exact: true }).fill('저장 전 직책');
		await selectOrganizationInTree(page, 'design');

		await expect(page.getByText('저장하지 않은 조직도 변경사항이 있습니다.')).toBeVisible();
		await expect(detailPanel.getByLabel('직책', { exact: true })).toHaveValue('저장 전 직책');
		await expect(page.getByTestId('organization-person-node-user-dabin')).toBeVisible();
	});

	test('keeps an unsaved edit draft when closing the detail panel', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 800 });
		await mockOrganizationDirectory(page, { canManage: true });

		await page.goto('/organization/');
		await page.getByTestId('organization-person-node-user-dabin').click();

		const detailPanel = page.getByTestId('organization-person-detail-panel');
		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await detailPanel.getByLabel('직책', { exact: true }).fill('닫기 전 직책');
		await detailPanel.getByRole('button', { name: '상세 닫기' }).click();

		await expect(page.getByText('저장하지 않은 조직도 변경사항이 있습니다.')).toBeVisible();
		await expect(detailPanel).toBeVisible();
		await expect(detailPanel.getByLabel('직책', { exact: true })).toHaveValue('닫기 전 직책');

		await detailPanel.getByRole('button', { name: '취소' }).click();
		await detailPanel.getByRole('button', { name: '상세 닫기' }).click();
		await expect(page.getByTestId('organization-person-detail-panel')).toHaveCount(0);
	});

	test('keeps an unsaved edit draft when search hides the selected person from the list', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 800 });
		await mockOrganizationDirectory(page, { canManage: true });

		await page.goto('/organization/');
		await page.getByTestId('organization-person-node-user-dabin').click();

		const detailPanel = page.getByTestId('organization-person-detail-panel');
		await detailPanel.getByRole('button', { name: '수정하기' }).click();
		await detailPanel.getByLabel('직책', { exact: true }).fill('검색 중 직책');
		await page.getByLabel('검색').fill('박지은');

		await expect(page.getByTestId('organization-person-node-user-dabin')).toHaveCount(0);
		await expect(detailPanel).toBeVisible();
		await expect(detailPanel).toContainText('김다빈');
		await expect(detailPanel.getByLabel('직책', { exact: true })).toHaveValue('검색 중 직책');
		await expect(detailPanel.getByRole('button', { name: '저장' })).toBeVisible();
		await expect(detailPanel.getByRole('button', { name: '취소' })).toBeVisible();
	});
});
