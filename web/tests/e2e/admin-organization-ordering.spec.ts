import { expect, test } from '@playwright/test';
import { cloneUsersResponse, expectCardBefore, mockAdminOrganization, openOrganizationEditor } from './admin-organization-helpers';
import { initialUsersResponse } from './admin-organization-fixtures';

test.describe('admin org chart ordering', () => {
	test('orders reports by hire date before name', async ({ page }) => {
		const usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.records = [
			usersResponse.records[0],
			{
				...usersResponse.records[1],
				name: 'Aaron Analyst',
				hireDate: '2026-04-01',
				supervisorID: 'user-ada',
				groupID: 'engineering'
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
				groupID: 'engineering'
			}
		];
		await mockAdminOrganization(page, {
			getUsersResponse: () => usersResponse
		});

		await openOrganizationEditor(page);

		await expectCardBefore(page, 'organization-profile-user-zara', 'organization-profile-user-grace');
	});

	test('orders fallback profiles by hire date', async ({ page }) => {
		const usersResponse = cloneUsersResponse(initialUsersResponse);
		usersResponse.records = [
			{
				...usersResponse.records[0],
				hireDate: '2026-03-10',
				supervisorID: '',
				groupID: ''
			},
			{
				...usersResponse.records[1],
				hireDate: '2026-02-01',
				supervisorID: '',
				groupID: ''
			}
		];
		await mockAdminOrganization(page, {
			getUsersResponse: () => usersResponse
		});

		await openOrganizationEditor(page);

		await expectCardBefore(page, 'organization-profile-user-grace', 'organization-profile-user-ada');
	});

	test('keeps reports after their direct manager in the organization list', async ({ page }) => {
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
				supervisorID: 'user-ada',
				groupID: 'engineering'
			},
			{
				userID: 'user-linus',
				handle: 'linus',
				name: 'Linus Park',
				email: 'linus@example.com',
				hireDate: '2026-01-01',
				role: 'member',
				jobTitle: 'Engineer',
				supervisorID: 'user-grace',
				groupID: 'engineering'
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
				groupID: 'operations'
			}
		];
		await mockAdminOrganization(page, {
			getUsersResponse: () => usersResponse
		});

		await openOrganizationEditor(page);

		const engineeringMembers = page.getByTestId('organization-members-engineering');
		await expect(engineeringMembers.getByTestId('organization-person-node-user-grace')).toBeVisible();
		await expect(engineeringMembers.getByTestId('organization-person-node-user-linus')).toBeVisible();
		await expect(engineeringMembers.getByTestId('organization-person-node-user-dan')).toHaveCount(0);
		await expectCardBefore(page, 'organization-profile-user-grace', 'organization-profile-user-linus');
	});
});
