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
			mattermostURL: 'https://demo.example.test'
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
		expect(body.records?.length).toBe(20);
		expect(body.availableGroups?.some((group) => group.id === 'group-product')).toBe(true);
	});

	test('returns public orgchart directory data for the employee page', () => {
		const state = createDevAdminOrgchartMockState('admin@example.com');
		state.groups.push({ id: 'group-engineering', name: '개발팀', parentID: 'group-product' });
		const response = createDevAdminOrgchartMockResponse(state, {
			method: 'GET',
			pathname: '/orgchart/api/people',
			searchParams: new URLSearchParams()
		});

		expect(response?.status).toBe(200);
		const body = response?.body as UsersResponse;
		const userIDs = body.records?.map((record) => record.userID);
		expect(userIDs?.length).toBe(20);
		expect(userIDs?.includes('dev-user-ceo')).toBe(true);
		expect(userIDs?.includes('dev-user-dabin')).toBe(true);
		expect(userIDs?.includes('dev-user-nam')).toBe(true);
		expect(body.availableGroups?.map((group) => group.id)).toEqual([
			'group-leadership',
			'group-operations',
			'group-product',
			'group-design',
			'group-field',
			'group-engineering'
		]);
		expect(body.availableGroups?.find((group) => group.id === 'group-design')?.parentID).toBe('group-product');
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

	test('preserves organization parent relationships when saving', () => {
		const state = createDevAdminOrgchartMockState('admin@example.com');
		const response = createDevAdminOrgchartMockResponse(state, {
			method: 'PUT',
			pathname: '/admin/api/org-groups',
			searchParams: new URLSearchParams('includePolicy=true'),
			body: JSON.stringify({
				groups: [
					{ id: 'group-product', name: '제품팀' },
					{ id: 'group-engineering', name: '개발팀', parentID: 'group-product' }
				]
			})
		});

		expect(response?.status).toBe(200);
		const body = response?.body as UsersResponse;
		expect(body.availableGroups).toEqual([
			{ id: 'group-product', name: '제품팀' },
			{ id: 'group-engineering', name: '개발팀', parentID: 'group-product' }
		]);
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
						userID: 'dev-user-dabin',
						email: 'dabin@example.com',
						jobTitle: 'Operations Lead',
						primaryGroupID: 'group-operations',
						groupIDs: ['group-operations'],
						supervisorID: 'dev-user-sujin'
					}
				]
			})
		});

		expect(response?.status).toBe(200);
		const body = response?.body as UsersResponse;
		const updatedUser = body.records?.find((record) => record.userID === 'dev-user-dabin');
		expect(updatedUser).toMatchObject({
			jobTitle: 'Operations Lead',
			primaryGroupID: 'group-operations',
			groupIDs: ['group-operations'],
			supervisorID: 'dev-user-sujin'
		});
	});
});
