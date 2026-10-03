import { describe, expect, test } from 'bun:test';
import { homesOfPersonUpdate } from '$lib/server/public-api/record/people-tools';
import { WorkspaceEmploymentStatus } from '$lib/server/public-api/catalog/people';
import { memberStatuses } from '$lib/member-vocabulary';

// The same list internal/admind/organization_source_boundary_test.go holds the
// fleet account payload to. A field on it belongs to organization_profiles, so
// person_update must never carry it to the account directory.
const fieldsOrganizationProfilesOwn = [
	'hireDate',
	'jobTitle',
	'groupID',
	'phoneNumber',
	'supervisorID',
	'positionLevel',
	'teamRole',
	'employmentStatus'
];

const everyFieldSet = {
	personHint: 'm1',
	name: '이샘플',
	isAdmin: true,
	jobTitle: '편집장',
	teamHint: '개발팀',
	supervisorHint: '박예시',
	phoneNumber: '010-0000-0000',
	hireDate: '2026-01-02',
	employmentStatus: 'departed'
};

describe('which home each field of a person update is written to', () => {
	test('sends the account directory the name and whether they administer, and no HR attribute', () => {
		const homes = homesOfPersonUpdate(everyFieldSet, { teamID: 't1', supervisorID: 'm2' });
		expect(Object.keys(homes.account).sort()).toEqual(['isAdmin', 'name']);
	});

	test('sends the organization profile every human-resources attribute', () => {
		const homes = homesOfPersonUpdate(everyFieldSet, { teamID: 't1', supervisorID: 'm2' });
		expect(Object.keys(homes.organization).sort()).toEqual([
			'employmentStatus',
			'groupID',
			'hireDate',
			'jobTitle',
			'phoneNumber',
			'supervisorID'
		]);
	});

	test('never carries an organization field to the account directory', () => {
		const homes = homesOfPersonUpdate(everyFieldSet, { teamID: 't1', supervisorID: 'm2' });
		for (const field of fieldsOrganizationProfilesOwn) {
			expect(Object.hasOwn(homes.account, field)).toBe(false);
		}
	});

	test('resolves the two hints into the identifiers the record stores', () => {
		const homes = homesOfPersonUpdate(everyFieldSet, { teamID: 't1', supervisorID: 'm2' });
		expect(homes.organization.groupID).toBe('t1');
		expect(homes.organization.supervisorID).toBe('m2');
		expect(Object.hasOwn(homes.organization, 'teamHint')).toBe(false);
		expect(Object.hasOwn(homes.organization, 'supervisorHint')).toBe(false);
	});

	test('writes only the fields the call named', () => {
		const homes = homesOfPersonUpdate({ personHint: 'm1', jobTitle: '편집장' }, {});
		expect(homes.account).toEqual({});
		expect(homes.organization).toEqual({ jobTitle: '편집장' });
	});

	test('keeps an emptied field as a change rather than dropping it', () => {
		const homes = homesOfPersonUpdate({ personHint: 'm1', phoneNumber: '' }, { teamID: '' });
		expect(homes.organization).toEqual({ groupID: '', phoneNumber: '' });
	});
});

describe('the employment statuses a person update may set', () => {
	test('are ones the central plane declares, so the two lists cannot drift apart', () => {
		for (const status of Object.values(WorkspaceEmploymentStatus)) {
			expect(memberStatuses).toContain(status);
		}
	});
});
