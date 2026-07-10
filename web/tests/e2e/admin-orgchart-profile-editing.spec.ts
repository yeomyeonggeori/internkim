import { expect, test } from '@playwright/test';
import { applySavedProfiles, cloneUsersResponse, enableOrgchartEditMode, mockAdminOrgchart, openCardEditor, openOrgchartEditor, openOrganizationForm, selectCardOption } from './admin-orgchart-helpers';
import { initialUsersResponse, type OrgProfileUpdate } from './admin-orgchart-fixtures';

test.describe('admin org chart profile editing', () => {
	test('moves the detail panel editor between selected people', async ({ page }) => {
		await mockAdminOrgchart(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse)
		});

		await openOrgchartEditor(page);

		const graceCard = await openCardEditor(page, 'user-grace');
		await expect(graceCard.getByLabel('직책', { exact: true })).toBeVisible();

		const adaCard = await openCardEditor(page, 'user-ada');
		await expect(adaCard.getByLabel('직책', { exact: true })).toBeVisible();
		await expect(page.getByTestId('orgchart-profile-user-grace')).toHaveCount(0);
	});

	test('keeps other editors and organization creation available while one profile saves', async ({ page }) => {
		let isSaveStarted = false;
		let resolveSave: () => void = () => {};
		await mockAdminOrgchart(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse),
			saveProfiles: async () => {
				isSaveStarted = true;
				await new Promise<void>((resolve) => {
					resolveSave = resolve;
				});
				return cloneUsersResponse(initialUsersResponse);
			}
		});

		await openOrgchartEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');
		await graceCard.getByLabel('직책', { exact: true }).fill('Saving title');
		await openOrganizationForm(page);
		await expect(graceCard.getByLabel('직책', { exact: true })).toHaveValue('Saving title');
		await graceCard.getByRole('button', { name: '저장' }).click();
		await expect.poll(() => isSaveStarted).toBe(true);

		await expect(graceCard.getByLabel('직책', { exact: true })).toBeDisabled();
		await expect(page.getByLabel('새 조직')).toBeEnabled();

		resolveSave();
		await expect(page.getByTestId('orgchart-profile-user-grace')).toHaveCount(0);
	});

	test('saves minimal organization metadata from existing user candidates', async ({ page }) => {
		const savedProfiles: OrgProfileUpdate[] = [];
		let usersResponse = cloneUsersResponse(initialUsersResponse);
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse,
			saveProfiles: async (profiles) => {
				savedProfiles.push(...profiles);
				usersResponse = applySavedProfiles(usersResponse, profiles);
				return usersResponse;
			}
		});

		await openOrgchartEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await selectCardOption(page, graceCard, '소속 조직', 'Engineering');
		await selectCardOption(page, graceCard, '직속 상관', 'Ada Kim');
		await graceCard.getByRole('button', { name: '저장' }).click();

		await expect.poll(() => savedProfiles).toEqual([
			expect.objectContaining({
				userID: 'user-grace',
				email: 'grace@example.com',
				jobTitle: 'Product Designer',
				primaryGroupID: 'engineering',
				groupIDs: ['engineering'],
				supervisorID: 'user-ada'
			})
		]);
	});

	test('does not allow selecting descendants as direct managers', async ({ page }) => {
		const usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.records = [
			{
				...usersResponse.records[0],
				userID: 'user-ada',
				name: 'Ada Kim',
				supervisorID: ''
			},
			{
				...usersResponse.records[1],
				userID: 'user-grace',
				name: 'Grace Lee',
				supervisorID: 'user-ada'
			},
			{
				userID: 'user-linus',
				handle: 'linus',
				name: 'Linus Park',
				email: 'linus@example.com',
				hireDate: '2026-03-01',
				role: 'member',
				jobTitle: 'Engineer',
				supervisorID: 'user-grace',
				primaryGroupID: 'engineering',
				groupIDs: ['engineering']
			},
			{
				userID: 'user-dan',
				handle: 'dan',
				name: 'Dan Root',
				email: 'dan@example.com',
				hireDate: '2026-01-20',
				role: 'member',
				jobTitle: 'Lead',
				supervisorID: '',
				primaryGroupID: 'operations',
				groupIDs: ['operations']
			}
		];
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse
		});

		await openOrgchartEditor(page);
		const adaCard = await openCardEditor(page, 'user-ada');
		await adaCard.getByLabel('직속 상관').click();

		await expect(page.getByRole('option', { name: 'Grace Lee' })).toHaveCount(0);
		await expect(page.getByRole('option', { name: 'Linus Park' })).toHaveCount(0);
		await expect(page.getByRole('option', { name: 'Dan Root' })).toBeVisible();
	});

	test('cancels a single profile edit without saving draft changes', async ({ page }) => {
		const savedProfiles: OrgProfileUpdate[] = [];
		await mockAdminOrgchart(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse),
			saveProfiles: async (profiles) => {
				savedProfiles.push(...profiles);
				return cloneUsersResponse(initialUsersResponse);
			}
		});

		await openOrgchartEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Draft title');
		await graceCard.getByRole('button', { name: '취소' }).click();

		await expect.poll(() => savedProfiles).toEqual([]);
		await expect(page.getByTestId('orgchart-profile-user-grace')).toHaveCount(0);
		const reopenedGraceCard = await openCardEditor(page, 'user-grace');
		await expect(reopenedGraceCard.getByLabel('직책', { exact: true })).toHaveValue('');
	});

	test('keeps unsaved direct manager edits visible when selecting another person', async ({ page }) => {
		const savedProfiles: OrgProfileUpdate[] = [];
		const usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.records = usersResponse.records.map((record) =>
			record.userID === 'user-grace'
				? { ...record, primaryGroupID: 'operations', groupIDs: ['operations'] }
				: record
		);
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse,
			saveProfiles: async (profiles) => {
				savedProfiles.push(...profiles);
				return usersResponse;
			}
		});

		await openOrgchartEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await selectCardOption(page, graceCard, '직속 상관', 'Ada Kim');
		await page.getByTestId('orgchart-person-node-user-ada').click();

		await expect(page.getByText('저장하지 않은 조직도 변경사항이 있습니다.')).toBeVisible();
		await expect(graceCard.getByLabel('직속 상관')).toContainText('Ada Kim');
		await expect(graceCard.getByLabel('소속 조직')).toContainText('Operations');
		await expect.poll(() => savedProfiles).toEqual([]);
	});

	test('keeps saved organization metadata after reload', async ({ page }) => {
		let usersResponse = cloneUsersResponse(initialUsersResponse);
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse,
			saveProfiles: async (profiles) => {
				usersResponse = applySavedProfiles(usersResponse, profiles);
				return usersResponse;
			}
		});

		await openOrgchartEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await selectCardOption(page, graceCard, '소속 조직', 'Engineering');
		await selectCardOption(page, graceCard, '직속 상관', 'Ada Kim');
		await graceCard.getByRole('button', { name: '저장' }).click();
		await expect(page.getByTestId('orgchart-profile-user-grace')).toHaveCount(0);

		await page.reload();
		await enableOrgchartEditMode(page);

		const reloadedGraceCard = await openCardEditor(page, 'user-grace');
		await expect(reloadedGraceCard.getByLabel('직책', { exact: true })).toHaveValue('Product Designer');
		await expect(reloadedGraceCard.getByLabel('소속 조직')).toContainText('Engineering');
		await expect(reloadedGraceCard.getByLabel('직속 상관')).toContainText('Ada Kim');
	});

	test('reflects saved admin org chart changes on the public org chart page', async ({ page }) => {
		let usersResponse = cloneUsersResponse(initialUsersResponse);
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse,
			saveProfiles: async (profiles) => {
				usersResponse = applySavedProfiles(usersResponse, profiles);
				return usersResponse;
			}
		});
		await page.route('**/orgchart/api/people', async (route) => {
			await route.fulfill({ json: usersResponse });
		});

		await openOrgchartEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await selectCardOption(page, graceCard, '소속 조직', 'Engineering');
		await selectCardOption(page, graceCard, '직속 상관', 'Ada Kim');
		await graceCard.getByRole('button', { name: '저장' }).click();
		await expect(page.getByTestId('orgchart-profile-user-grace')).toHaveCount(0);

		await page.goto('/orgchart/');

		await expect(page.getByTestId('orgchart-person-node-user-ada')).toBeVisible();
		await expect(page.getByTestId('orgchart-team-column-engineering')).toBeVisible();
		await expect(page.getByTestId('orgchart-tree-node-user-grace')).toBeVisible();
		await expect(page.getByTestId('orgchart-team-column-__unassigned__')).toHaveCount(0);
	});

	test('keeps failed saves visible', async ({ page }) => {
		await mockAdminOrgchart(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse),
			saveProfiles: async () => {
				throw new Error('사용자 저장에 실패했습니다.');
			}
		});

		await openOrgchartEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Designer');
		await graceCard.getByRole('button', { name: '저장' }).click();

		await expect(page.getByText('사용자 저장에 실패했습니다.')).toBeVisible();
		await expect(graceCard.getByRole('button', { name: '저장' })).toBeEnabled();
		await graceCard.getByRole('button', { name: '취소' }).click();
		await expect(page.getByText('사용자 저장에 실패했습니다.')).toHaveCount(0);
	});
});
