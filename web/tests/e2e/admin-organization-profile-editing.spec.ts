import { expect, test } from '@playwright/test';
import { applySavedProfiles, cardEditorButton, cloneUsersResponse, enableOrganizationEditMode, mockAdminOrganization, openCardEditor, openOrganizationEditor, openOrganizationForm, selectCardOption } from './admin-organization-helpers';
import { initialUsersResponse, type OrgProfileUpdate } from './admin-organization-fixtures';

test.describe('admin org chart profile editing', () => {
	test('moves the detail panel editor between selected people', async ({ page }) => {
		await mockAdminOrganization(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse)
		});

		await openOrganizationEditor(page);

		const graceCard = await openCardEditor(page, 'user-grace');
		await expect(graceCard.getByLabel('직책', { exact: true })).toBeVisible();

		const adaCard = await openCardEditor(page, 'user-ada');
		await expect(adaCard.getByLabel('직책', { exact: true })).toBeVisible();
		await expect(page.getByTestId('organization-profile-user-grace')).toHaveCount(0);
	});

	test('saves minimal organization metadata from existing user candidates', async ({ page }) => {
		const savedProfiles: OrgProfileUpdate[] = [];
		let usersResponse = cloneUsersResponse(initialUsersResponse);
		await mockAdminOrganization(page, {
			getUsersResponse: () => usersResponse,
			saveProfiles: async (profiles) => {
				savedProfiles.push(...profiles);
				usersResponse = applySavedProfiles(usersResponse, profiles);
				return usersResponse;
			}
		});

		await openOrganizationEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await selectCardOption(page, graceCard, '소속 조직', 'Engineering');
		await selectCardOption(page, graceCard, '직속 상관', 'Ada Kim');
		await cardEditorButton(page, '저장').click();

		await expect.poll(() => savedProfiles).toEqual([
			expect.objectContaining({
				memberID: 'user-grace',
				email: 'grace@example.com',
				jobTitle: 'Product Designer',
				groupID: 'engineering',
				supervisorID: 'user-ada'
			})
		]);
	});

	test('does not allow selecting descendants as direct managers', async ({ page }) => {
		const usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.records = [
			{
				...usersResponse.records[0],
				memberID: 'user-ada',
				name: 'Ada Kim',
				supervisorID: ''
			},
			{
				...usersResponse.records[1],
				memberID: 'user-grace',
				name: 'Grace Lee',
				supervisorID: 'user-ada'
			},
			{
				memberID: 'user-linus',
				handle: 'linus',
				name: 'Linus Park',
				email: 'linus@example.com',
				hireDate: '2026-03-01',
				role: 'member',
				jobTitle: 'Engineer',
				supervisorID: 'user-grace',
				groupID: 'engineering'
			},
			{
				memberID: 'user-dan',
				handle: 'dan',
				name: 'Dan Root',
				email: 'dan@example.com',
				hireDate: '2026-01-20',
				role: 'member',
				jobTitle: 'Lead',
				supervisorID: '',
				groupID: 'operations'
			}
		];
		await mockAdminOrganization(page, {
			getUsersResponse: () => usersResponse
		});

		await openOrganizationEditor(page);
		const adaCard = await openCardEditor(page, 'user-ada');
		await adaCard.getByLabel('직속 상관').click();

		await expect(page.getByRole('option', { name: 'Grace Lee' })).toHaveCount(0);
		await expect(page.getByRole('option', { name: 'Linus Park' })).toHaveCount(0);
		await expect(page.getByRole('option', { name: 'Dan Root' })).toBeVisible();
	});

	test('cancels a single profile edit without saving draft changes', async ({ page }) => {
		const savedProfiles: OrgProfileUpdate[] = [];
		await mockAdminOrganization(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse),
			saveProfiles: async (profiles) => {
				savedProfiles.push(...profiles);
				return cloneUsersResponse(initialUsersResponse);
			}
		});

		await openOrganizationEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Draft title');
		await cardEditorButton(page, '취소').click();

		await expect.poll(() => savedProfiles).toEqual([]);
		await expect(page.getByTestId('organization-profile-user-grace')).toHaveCount(0);
		const reopenedGraceCard = await openCardEditor(page, 'user-grace');
		await expect(reopenedGraceCard.getByLabel('직책', { exact: true })).toHaveValue('');
	});

	test('keeps saved organization metadata after reload', async ({ page }) => {
		let usersResponse = cloneUsersResponse(initialUsersResponse);
		await mockAdminOrganization(page, {
			getUsersResponse: () => usersResponse,
			saveProfiles: async (profiles) => {
				usersResponse = applySavedProfiles(usersResponse, profiles);
				return usersResponse;
			}
		});

		await openOrganizationEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await selectCardOption(page, graceCard, '소속 조직', 'Engineering');
		await selectCardOption(page, graceCard, '직속 상관', 'Ada Kim');
		await cardEditorButton(page, '저장').click();
		await expect(page.getByTestId('organization-profile-user-grace')).toHaveCount(0);

		await page.reload();
		await enableOrganizationEditMode(page);

		const reloadedGraceCard = await openCardEditor(page, 'user-grace');
		await expect(reloadedGraceCard.getByLabel('직책', { exact: true })).toHaveValue('Product Designer');
		await expect(reloadedGraceCard.getByLabel('소속 조직')).toContainText('Engineering');
		await expect(reloadedGraceCard.getByLabel('직속 상관')).toContainText('Ada Kim');
	});

	test('reflects saved admin org chart changes on the public org chart page', async ({ page }) => {
		let usersResponse = cloneUsersResponse(initialUsersResponse);
		await mockAdminOrganization(page, {
			getUsersResponse: () => usersResponse,
			saveProfiles: async (profiles) => {
				usersResponse = applySavedProfiles(usersResponse, profiles);
				return usersResponse;
			}
		});
		await page.route('**/organization/api/people', async (route) => {
			await route.fulfill({ json: usersResponse });
		});

		await openOrganizationEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await selectCardOption(page, graceCard, '소속 조직', 'Engineering');
		await selectCardOption(page, graceCard, '직속 상관', 'Ada Kim');
		await cardEditorButton(page, '저장').click();
		await expect(page.getByTestId('organization-profile-user-grace')).toHaveCount(0);

		await page.goto('/organization/');

		await expect(page.getByTestId('organization-person-node-user-ada')).toBeVisible();
		const engineering = page.getByTestId('organization-members-engineering');
		await expect(engineering).toBeVisible();
		await expect(engineering.getByTestId('organization-person-node-user-grace')).toBeVisible();
		await expect(page.getByTestId('organization-members-__unassigned__')).toHaveCount(0);
	});

	test('keeps failed saves visible', async ({ page }) => {
		await mockAdminOrganization(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse),
			saveProfiles: async () => {
				throw new Error('사용자 저장에 실패했습니다.');
			}
		});

		await openOrganizationEditor(page);
		const graceCard = await openCardEditor(page, 'user-grace');

		await graceCard.getByLabel('직책', { exact: true }).fill('Designer');
		await cardEditorButton(page, '저장').click();

		await expect(page.getByText('사용자 저장에 실패했습니다.')).toBeVisible();
		await expect(cardEditorButton(page, '저장')).toBeEnabled();
		await cardEditorButton(page, '취소').click();
		await expect(page.getByText('사용자 저장에 실패했습니다.')).toHaveCount(0);
	});
});
