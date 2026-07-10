import { expect, test } from '@playwright/test';
import { applySavedProfiles, cloneUsersResponse, mockAdminOrgchart, openCardEditor, openOrgchartEditor, openOrganizationForm, selectCardOption } from './admin-orgchart-helpers';
import { initialUsersResponse, type OrgProfileUpdate } from './admin-orgchart-fixtures';

test.describe('admin org chart groups', () => {
	test('adds an organization from the organization picker', async ({ page }) => {
		const savedGroups: { id: string; name: string }[][] = [];
		let usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.availableGroups = [
			...usersResponse.availableGroups,
			{ id: 'engineering-duplicate', name: 'Engineering Duplicate' }
		];
		usersResponse.records = usersResponse.records.map((record) =>
			record.userID === 'user-grace'
				? { ...record, primaryGroupID: 'engineering-duplicate', groupIDs: ['engineering-duplicate'] }
				: record
		);
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse,
			saveGroups: async (groups) => {
				savedGroups.push(groups);
				const platformGroup = groups.find((group) => group.name === 'Platform');
				usersResponse = {
					...usersResponse,
					records: usersResponse.records.map((record) => {
						if (record.userID === 'user-ada' && platformGroup) return { ...record, primaryGroupID: platformGroup.id, groupIDs: [platformGroup.id] };
						if (record.userID === 'user-grace') return { ...record, primaryGroupID: 'engineering', groupIDs: ['engineering'] };
						return record;
					}),
					availableGroups: groups.filter((group) => group.id !== 'engineering-duplicate')
				};
				return usersResponse;
			}
		});

		await openOrgchartEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await openOrganizationForm(page);
		await page.getByLabel('새 조직').fill('Platform');
		await page.getByRole('button', { name: '추가', exact: true }).click();

		await expect.poll(() => savedGroups).toEqual([
			expect.arrayContaining([expect.objectContaining({ name: 'Platform' })])
		]);
		await expect(graceCard.getByLabel('직책', { exact: true })).toHaveValue('Product Designer');
		await expect(page.getByTestId('orgchart-organization-grid')).toContainText('Platform');
		await expect(graceCard.getByLabel('소속 조직')).toContainText('Engineering');
		await selectCardOption(page, graceCard, '소속 조직', 'Platform');
		await expect(graceCard.getByLabel('소속 조직')).toContainText('Platform');
	});

	test('reuses an existing organization when the new organization name already exists', async ({ page }) => {
		const savedGroups: { id: string; name: string }[][] = [];
		const savedProfiles: OrgProfileUpdate[] = [];
		let usersResponse = cloneUsersResponse(initialUsersResponse);
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse,
			saveGroups: async (groups) => {
				savedGroups.push(groups);
				usersResponse = { ...usersResponse, availableGroups: groups };
				return usersResponse;
			},
			saveProfiles: async (profiles) => {
				savedProfiles.push(...profiles);
				usersResponse = applySavedProfiles(usersResponse, profiles);
				return usersResponse;
			}
		});

		await openOrgchartEditor(page);
		await openOrganizationForm(page);
		await page.getByLabel('새 조직').fill(' Engineering ');
		await page.getByRole('button', { name: '추가', exact: true }).click();
		const graceCard = await openCardEditor(page, 'user-grace');
		await selectCardOption(page, graceCard, '소속 조직', 'Engineering');
		await graceCard.getByRole('button', { name: '저장' }).click();

		await expect.poll(() => savedGroups).toEqual([]);
		await expect.poll(() => savedProfiles).toEqual([
			expect.objectContaining({
				userID: 'user-grace',
				primaryGroupID: 'engineering',
				groupIDs: ['engineering']
			})
		]);
		await expect(page.getByTestId('orgchart-person-node-user-grace')).toBeVisible();
	});

	test('keeps the organization name visible when organization creation fails', async ({ page }) => {
		await mockAdminOrgchart(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse),
			saveGroups: async () => {
				throw new Error('사용자 저장에 실패했습니다.');
			}
		});

		await openOrgchartEditor(page);

		await openOrganizationForm(page);
		await page.getByLabel('새 조직').fill('Platform');
		await page.getByRole('button', { name: '추가', exact: true }).click();

		await expect(page.getByText('사용자 저장에 실패했습니다.')).toBeVisible();
		await expect(page.getByLabel('새 조직')).toHaveValue('Platform');
	});
});
