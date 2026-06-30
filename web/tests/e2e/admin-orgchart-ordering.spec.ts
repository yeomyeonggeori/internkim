import { expect, test } from '@playwright/test';
import { cloneUsersResponse, expectCardBefore, mockAdminOrgchart, openOrgchartEditor } from './admin-orgchart-helpers';
import { initialUsersResponse } from './admin-orgchart-fixtures';

test.describe('admin org chart ordering', () => {
	test('orders reports by hire date before name', async ({ page }) => {
		const usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.records = [
			usersResponse.records[0],
			{
				...usersResponse.records[1],
				name: 'Aaron Analyst',
				hireDate: '2026-04-01',
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
				supervisorID: 'user-ada',
				primaryGroupID: 'engineering',
				groupIDs: ['engineering']
			}
		];
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse
		});

		await openOrgchartEditor(page);

		await expectCardBefore(page, 'orgchart-profile-user-zara', 'orgchart-profile-user-grace');
	});

	test('orders fallback profiles by hire date', async ({ page }) => {
		const usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.records = [
			{
				...usersResponse.records[0],
				hireDate: '2026-03-10',
				supervisorID: 'user-grace'
			},
			{
				...usersResponse.records[1],
				hireDate: '2026-02-01',
				supervisorID: 'user-ada'
			}
		];
		await mockAdminOrgchart(page, {
			getUsersResponse: () => usersResponse
		});

		await openOrgchartEditor(page);

		await expectCardBefore(page, 'orgchart-profile-user-grace', 'orgchart-profile-user-ada');
	});

	test('groups reports under their direct manager', async ({ page }) => {
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

		const adaReports = page.getByTestId('orgchart-reports-user-ada');
		await expect(adaReports.getByTestId('orgchart-profile-user-grace')).toBeVisible();
		await expect(adaReports.getByTestId('orgchart-profile-user-linus')).toBeVisible();
		await expect(adaReports.getByTestId('orgchart-profile-user-dan')).toHaveCount(0);
		await expect(page.getByTestId('orgchart-reports-user-grace').getByTestId('orgchart-profile-user-linus')).toBeVisible();
	});
});
