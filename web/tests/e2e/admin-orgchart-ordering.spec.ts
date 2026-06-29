import { test } from '@playwright/test';
import { cloneUsersResponse, expectCardBefore, mockAdminOrgchart, openOrgchartEditor } from './admin-orgchart-helpers';
import { initialUsersResponse } from './admin-orgchart-fixtures';

test.describe('admin org chart ordering', () => {
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
});
