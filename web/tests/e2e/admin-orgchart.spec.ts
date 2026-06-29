import { expect, test } from '@playwright/test';
import { applySavedProfiles, cloneUsersResponse, expectCardBefore, mockAdminOrgchart, openCardEditor, openOrgchartEditor, selectCardOption, selectGroupMembership } from './admin-orgchart-helpers';
import { initialUsersResponse, type OrgProfileUpdate } from './admin-orgchart-fixtures';

test.describe('admin org chart editor', () => {
	test('orders reports by position level before name', async ({ page }) => {
		const usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.records = [
			usersResponse.records[0],
			{
				...usersResponse.records[1],
				name: 'Aaron Analyst',
				positionLevel: 5,
				supervisorID: 'user-ada'
			},
			{
				userID: 'user-zara',
				handle: 'zara',
				name: 'Zara Lead',
				email: 'zara@example.com',
				hireDate: '2026-02-04',
				role: 'member',
				jobTitle: 'Lead',
				positionLevel: 2,
				supervisorID: 'user-ada',
				primaryGroupID: 'engineering',
				groupIDs: ['engineering'],
				projectIDs: [],
				employmentStatus: 'active',
				isOrgchartVisible: true
			}
		];
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse
		});

		await openOrgchartEditor(page);

		await expectCardBefore(page, 'orgchart-profile-user-zara', 'orgchart-profile-user-grace');
	});

	test('orders fallback profiles by position level', async ({ page }) => {
		const usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.records = [
			{
				...usersResponse.records[0],
				positionLevel: 5,
				supervisorID: 'user-grace'
			},
			{
				...usersResponse.records[1],
				positionLevel: 1,
				supervisorID: 'user-ada'
			}
		];
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse
		});

		await openOrgchartEditor(page);

		await expectCardBefore(page, 'orgchart-profile-user-grace', 'orgchart-profile-user-ada');
	});

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

	test('adds an organization from the primary organization picker', async ({ page }) => {
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
		const graceCard = page.getByTestId('orgchart-profile-user-grace');
		await openCardEditor(graceCard);

		await graceCard.getByLabel('직책', { exact: true }).fill('Product Designer');
		await page.getByLabel('새 조직').fill('Platform');
		await page.getByRole('button', { name: '조직 추가' }).click();

		await expect.poll(() => savedGroups).toEqual([
			expect.arrayContaining([expect.objectContaining({ name: 'Platform' })])
		]);
		await expect(graceCard.getByLabel('직책', { exact: true })).toHaveValue('Product Designer');
		await expect(page.getByTestId('orgchart-profile-user-ada')).toContainText('Platform');
		await expect(graceCard.getByLabel('주 소속 조직')).toContainText('Engineering');
		await selectCardOption(page, graceCard, '주 소속 조직', 'Platform');
		await expect(graceCard.getByLabel('주 소속 조직')).toContainText('Platform');
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
		await page.getByLabel('새 조직').fill(' Engineering ');
		await page.getByRole('button', { name: '조직 추가' }).click();
		const graceCard = page.getByTestId('orgchart-profile-user-grace');
		await openCardEditor(graceCard);
		await selectCardOption(page, graceCard, '주 소속 조직', 'Engineering');
		await graceCard.getByRole('button', { name: '저장' }).click();

		await expect.poll(() => savedGroups).toEqual([]);
		await expect.poll(() => savedProfiles).toEqual([
			expect.objectContaining({
				userID: 'user-grace',
				primaryGroupID: 'engineering',
				groupIDs: ['engineering']
			})
		]);
		await expect(graceCard).toContainText('Engineering');
	});

	test('keeps the organization name visible when organization creation fails', async ({ page }) => {
		await mockAdminOrgchart(page, {
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse),
			saveGroups: async () => {
				throw new Error('사용자 저장에 실패했습니다.');
			}
		});

		await openOrgchartEditor(page);

		await page.getByLabel('새 조직').fill('Platform');
		await page.getByRole('button', { name: '조직 추가' }).click();

		await expect(page.getByText('사용자 저장에 실패했습니다.')).toBeVisible();
		await expect(page.getByLabel('새 조직')).toHaveValue('Platform');
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

	test('falls back from hidden device-only section query to users', async ({ page }) => {
		await mockAdminOrgchart(page, {
			deviceManaged: false,
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse)
		});

		for (const section of ['device', 'network']) {
			await page.goto(`/admin/?fleet_id=demo&section=${section}`);

			await expect(page.getByRole('button', { name: '기기' })).toHaveCount(0);
			await expect(page.getByRole('button', { name: '네트워크' })).toHaveCount(0);
			await expect(page.getByRole('heading', { name: '허용된 사용자' })).toBeVisible();
			await expect(page.getByText('WiFi')).toHaveCount(0);
		}
	});
});
