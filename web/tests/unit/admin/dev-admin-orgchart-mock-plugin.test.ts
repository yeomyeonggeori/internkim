import { describe, expect, test } from 'bun:test';
import {
	createDevAdminOrgchartMockResponse,
	createDevAdminOrgchartMockState
} from '../../../dev-admin-orgchart-state';
import type { UsersResponse } from '../../../src/routes/admin/admin-types';

describe('dev admin orgchart mock plugin', () => {
	test('returns a device managed admin session', () => {
		const state = createDevAdminOrgchartMockState('admin@example.com');
		const response = createDevAdminOrgchartMockResponse(state, {
			method: 'GET',
			pathname: '/admin/api/session',
			searchParams: new URLSearchParams()
		});

		expect(response?.status).toBe(200);
		expect(response?.body).toMatchObject({
			email: 'admin@example.com',
			isAdmin: true,
			deviceManaged: true,
			mattermostURL: 'https://demo.intern.kim'
		});
	});

	test('returns users and available groups for the org chart tab', () => {
		const state = createDevAdminOrgchartMockState('admin@example.com');
		const response = createDevAdminOrgchartMockResponse(state, {
			method: 'GET',
			pathname: '/admin/api/users',
			searchParams: new URLSearchParams('includePolicy=true')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as UsersResponse;
		expect(body.records?.length).toBe(3);
		expect(body.availableGroups?.some((group) => group.id === 'group-engineering')).toBe(true);
	});

	test('deduplicates groups by case-insensitive name when saving', () => {
		const state = createDevAdminOrgchartMockState('admin@example.com');
		const response = createDevAdminOrgchartMockResponse(state, {
			method: 'PUT',
			pathname: '/admin/api/org-groups',
			searchParams: new URLSearchParams('includePolicy=true'),
			body: JSON.stringify({
				groups: [
					{ id: 'group-engineering', name: 'Engineering' },
					{ id: 'group-duplicate', name: ' engineering ' },
					{ id: 'group-design', name: 'Design' }
				]
			})
		});

		expect(response?.status).toBe(200);
		const body = response?.body as UsersResponse;
		expect(body.availableGroups?.map((group) => group.name)).toEqual(['Engineering', 'Design']);
	});

	test('applies org profile updates to mock users', () => {
		const state = createDevAdminOrgchartMockState('admin@example.com');
		const response = createDevAdminOrgchartMockResponse(state, {
			method: 'POST',
			pathname: '/admin/api/users/org-profiles',
			searchParams: new URLSearchParams('includePolicy=true'),
			body: JSON.stringify({
				profiles: [
					{
						userID: 'dev-user-grace',
						email: 'grace@example.com',
						jobTitle: 'Operations Lead',
						primaryGroupID: 'group-operations',
						groupIDs: ['group-operations'],
						supervisorID: 'dev-user-ada'
					}
				]
			})
		});

		expect(response?.status).toBe(200);
		const body = response?.body as UsersResponse;
		const updatedUser = body.records?.find((record) => record.userID === 'dev-user-grace');
		expect(updatedUser).toMatchObject({
			jobTitle: 'Operations Lead',
			primaryGroupID: 'group-operations',
			groupIDs: ['group-operations'],
			supervisorID: 'dev-user-ada',
			projectIDs: ['blueclaw', 'admin'],
			teamRole: 'frontend',
			employmentStatus: 'active',
			isOrgchartVisible: true
		});
	});
});
