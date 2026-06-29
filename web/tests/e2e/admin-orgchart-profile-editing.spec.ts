import { expect, test } from '@playwright/test';
import { applySavedProfiles, cloneUsersResponse, mockAdminOrgchart, openCardEditor, openOrgchartEditor, selectCardOption, selectGroupMembership } from './admin-orgchart-helpers';
import { initialUsersResponse, type OrgProfileUpdate } from './admin-orgchart-fixtures';

test.describe('admin org chart profile editing', () => {
	test('opens multiple profile editors from edit buttons', async ({ page }) => {
		await mockAdminOrgchart(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse)
		});

		await openOrgchartEditor(page);
		const adaCard = page.getByTestId('orgchart-profile-user-ada');
		const graceCard = page.getByTestId('orgchart-profile-user-grace');

		await openCardEditor(graceCard);

		await openCardEditor(adaCard);
		await expect(adaCard.getByLabel('직책', { exact: true })).toBeVisible();
		await expect(graceCard.getByLabel('직책', { exact: true })).toBeVisible();
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
		const adaCard = page.getByTestId('orgchart-profile-user-ada');
		const graceCard = page.getByTestId('orgchart-profile-user-grace');
		await openCardEditor(graceCard);
		await openCardEditor(adaCard);

		await graceCard.getByLabel('직책', { exact: true }).fill('Saving title');
		await graceCard.getByRole('button', { name: '저장' }).click();
		await expect.poll(() => isSaveStarted).toBe(true);

		await expect(graceCard.getByLabel('직책', { exact: true })).toBeDisabled();
		await expect(adaCard.getByLabel('직책', { exact: true })).toBeEnabled();
		await expect(page.getByLabel('새 조직')).toBeEnabled();

		resolveSave();
		await expect(graceCard.getByRole('button', { name: '편집' })).toBeVisible();
	});

	test('saves full organization metadata from existing user candidates', async ({ page }) => {
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
		const graceCard = page.getByTestId('orgchart-profile-user-grace');
		await openCardEditor(graceCard);

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await graceCard.getByLabel('직책 레벨').fill('4');
		await selectCardOption(page, graceCard, '주 소속 조직', 'Engineering');
		await selectGroupMembership(page, graceCard, 'Operations');
		await selectCardOption(page, graceCard, '상위 담당자', 'Ada Kim');
		await graceCard.getByLabel('담당 프로젝트').fill('Brand refresh');
		await graceCard.getByLabel('담당 프로젝트').press('Enter');
		await graceCard.getByLabel('팀 안 역할').fill('Design systems');
		await selectCardOption(page, graceCard, '재직 상태', '휴직');
		await selectCardOption(page, graceCard, '조직도 표시 여부', '숨김');
		await graceCard.getByRole('button', { name: '저장' }).click();

		await expect.poll(() => savedProfiles).toEqual([
			expect.objectContaining({
				userID: 'user-grace',
				email: 'grace@example.com',
				jobTitle: 'Product Designer',
				positionLevel: 4,
				primaryGroupID: 'engineering',
				groupIDs: ['engineering', 'operations'],
				supervisorID: 'user-ada',
				projectIDs: ['Brand refresh'],
				teamRole: 'Design systems',
				employmentStatus: 'leave',
				isOrgchartVisible: false
			})
		]);
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
		const graceCard = page.getByTestId('orgchart-profile-user-grace');
		await openCardEditor(graceCard);

		await graceCard.getByLabel('직책', { exact: true }).fill('Draft title');
		await graceCard.getByRole('button', { name: '취소' }).click();

		await expect.poll(() => savedProfiles).toEqual([]);
		await expect(graceCard.getByRole('button', { name: '편집' })).toBeVisible();
		await openCardEditor(graceCard);
		await expect(graceCard.getByLabel('직책', { exact: true })).toHaveValue('');
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
		const graceCard = page.getByTestId('orgchart-profile-user-grace');
		await openCardEditor(graceCard);

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await selectCardOption(page, graceCard, '주 소속 조직', 'Engineering');
		await graceCard.getByRole('button', { name: '저장' }).click();
		await expect(graceCard.getByRole('button', { name: '편집' })).toBeVisible();

		await page.reload();
		await page.getByLabel('편집').click();

		const reloadedGraceCard = page.getByTestId('orgchart-profile-user-grace');
		await openCardEditor(reloadedGraceCard);
		await expect(reloadedGraceCard.getByLabel('직책', { exact: true })).toHaveValue('Product Designer');
		await expect(reloadedGraceCard.getByLabel('주 소속 조직')).toContainText('Engineering');
	});

	test('keeps invalid position level and failed saves visible', async ({ page }) => {
		await mockAdminOrgchart(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse),
			saveProfiles: async () => {
				throw new Error('사용자 저장에 실패했습니다.');
			}
		});

		await openOrgchartEditor(page);
		const graceCard = page.getByTestId('orgchart-profile-user-grace');
		await openCardEditor(graceCard);

		await graceCard.getByLabel('직책 레벨').fill('0');
		await expect(graceCard.getByRole('button', { name: '저장' })).toBeDisabled();
		await expect(graceCard.getByText('1 이상의 숫자를 입력하세요.')).toBeVisible();

		await graceCard.getByLabel('직책 레벨').fill('-1');
		await expect(graceCard.getByRole('button', { name: '저장' })).toBeDisabled();
		await expect(graceCard.getByText('1 이상의 숫자를 입력하세요.')).toBeVisible();

		await graceCard.getByLabel('직책 레벨').fill('3');
		await graceCard.getByLabel('직책', { exact: true }).fill('Designer');
		await graceCard.getByRole('button', { name: '저장' }).click();

		await expect(page.getByText('사용자 저장에 실패했습니다.')).toBeVisible();
		await expect(graceCard.getByRole('button', { name: '저장' })).toBeEnabled();
		await graceCard.getByRole('button', { name: '취소' }).click();
		await expect(page.getByText('사용자 저장에 실패했습니다.')).toHaveCount(0);
	});
});
