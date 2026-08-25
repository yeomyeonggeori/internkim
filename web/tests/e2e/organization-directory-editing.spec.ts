import { expect, test } from '@playwright/test';
import {
	chooseInOrganizationMenu,
	closeDetailSheet,
	detailPanel,
	expectDetailSheetBesideTheList,
	expectDetailSheetStableWhileListScrolls,
	expectPersonDetailPanelContent,
	mockOrganizationDirectory
} from './organization-directory-helpers';

type SavedGroup = { id: string; name: string; parentID?: string };

test.describe('employee organization directory editing', () => {
	test('lets employees edit their own organization profile', async ({ page }) => {
		let savedProfile: { phoneNumber: string; hireDate: string } | undefined;
		await mockOrganizationDirectory(page);
		await page.route('**/organization/api/me/profile', async (route) => {
			savedProfile = route.request().postDataJSON() as { phoneNumber: string; hireDate: string };
			await route.fulfill({ json: savedProfile });
		});

		await page.goto('/organization/');
		await page.getByTestId('organization-person-node-user-specimen-choi').click();

		const panel = detailPanel(page);
		await panel.getByRole('button', { name: '수정하기' }).click();

		await expect(panel.getByLabel('전화번호')).toBeEnabled();
		await expect(panel.getByLabel('입사일')).toBeEnabled();
		await panel.getByLabel('전화번호').fill('+82 10-1234-5678');
		await panel.getByLabel('입사일').fill('2026-03-12');
		await panel.getByRole('button', { name: '저장', exact: true }).click();

		await expect.poll(() => savedProfile).toEqual({
			phoneNumber: '+82 10-1234-5678',
			hireDate: '2026-03-12'
		});
		await expect(panel).toContainText('+82 10-1234-5678');
		await expect(panel).toContainText('2026-03-12');

		await page.setViewportSize({ width: 390, height: 520 });
		await page.reload();
		await page.getByTestId('organization-person-node-user-specimen-choi').click();
		await panel.getByRole('button', { name: '수정하기' }).click();
		await expect(panel.getByLabel('전화번호')).toBeEnabled();
		await expect(panel.getByLabel('입사일')).toBeEnabled();
	});

	test('adds an organization under the selected parent', async ({ page }) => {
		let savedGroups: SavedGroup[] = [];
		await mockOrganizationDirectory(page, { canManage: true });
		await page.route('**/admin/api/org-groups**', async (route) => {
			const payload = route.request().postDataJSON() as { groups: SavedGroup[] };
			savedGroups = payload.groups;
			await route.fulfill({ json: { availableGroups: payload.groups, records: [] } });
		});
		await page.goto('/organization/');

		await chooseInOrganizationMenu(page, '조직 추가');
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
		await chooseInOrganizationMenu(page, '편집');

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

		await expect(page.getByTestId('organization-drop-preview')).toContainText('전체 바로 아래');
		await page.mouse.up();
		await sidebar.getByRole('button', { name: '저장', exact: true }).click();

		await expect.poll(() => savedGroups.find((group) => group.id === 'design')?.parentID ?? '').toBe('');
		await expect(sidebar.getByRole('button', { name: '저장', exact: true })).toHaveCount(0);
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
		await chooseInOrganizationMenu(page, '편집');

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
		await sidebar.getByRole('button', { name: '저장', exact: true }).click();

		await expect.poll(() => savedGroups.map((group) => group.id)).toEqual(['leadership', 'product', 'design', 'field']);
	});

	test('keeps the detail panel fixed while admins edit the selected person', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 420 });
		await mockOrganizationDirectory(page, { canManage: true });

		await page.goto('/organization/');

		await expect(page.getByRole('button', { name: '조직 작업' })).toBeVisible();
		await expect(page.getByTestId('organization-profile-user-specimen-choi')).toHaveCount(0);
		await expect(page.getByTestId('organization-section-product')).toBeVisible();

		await page.getByTestId('organization-person-node-user-specimen-choi').click();
		const panel = detailPanel(page);
		await expectPersonDetailPanelContent(panel);
		await expect(panel.getByRole('button', { name: '수정하기' })).toBeVisible();
		await expectDetailSheetBesideTheList(page);
		await expectDetailSheetStableWhileListScrolls(page);

		await panel.getByRole('button', { name: '수정하기' }).click();
		await expect(page.getByTestId('organization-profile-user-specimen-choi').getByLabel('직책', { exact: true })).toBeVisible();
	});

	test('asks before it throws away an unsaved edit, and keeps it when told to', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 800 });
		await mockOrganizationDirectory(page, { canManage: true });

		await page.goto('/organization/');
		await page.getByTestId('organization-person-node-user-specimen-choi').click();

		const panel = detailPanel(page);
		await panel.getByRole('button', { name: '수정하기' }).click();
		await panel.getByLabel('직책', { exact: true }).fill('닫기 전 직책');
		await closeDetailSheet(page);

		const dialog = page.getByTestId('organization-discard-edits-dialog');
		await expect(dialog).toBeVisible();
		await dialog.getByRole('button', { name: '계속 수정', exact: true }).click();
		await expect(dialog).toHaveCount(0);
		await expect(panel).toBeVisible();
		await expect(panel.getByLabel('직책', { exact: true })).toHaveValue('닫기 전 직책');
	});

	test('throws away an unsaved edit when told to close anyway', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 800 });
		await mockOrganizationDirectory(page, { canManage: true });

		await page.goto('/organization/');
		await page.getByTestId('organization-person-node-user-specimen-choi').click();

		const panel = detailPanel(page);
		await panel.getByRole('button', { name: '수정하기' }).click();
		await panel.getByLabel('직책', { exact: true }).fill('버릴 직책');
		await closeDetailSheet(page);

		await page.getByTestId('organization-discard-edits-dialog').getByRole('button', { name: '닫기', exact: true }).click();
		await expect(detailPanel(page)).toHaveCount(0);

		await page.getByTestId('organization-person-node-user-specimen-choi').click();
		await panel.getByRole('button', { name: '수정하기' }).click();
		await expect(panel.getByLabel('직책', { exact: true })).toHaveValue('프론트엔드 개발자');
	});
});
