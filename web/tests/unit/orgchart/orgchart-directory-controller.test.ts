import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/lib/organization/types';
import { adminText } from '../../../src/routes/admin/text';
import { unassignedGroupID } from '../../../src/routes/organization/organization-directory-model';
import { organizationDirectoryText } from '../../../src/routes/organization/text';

Object.assign(globalThis, {
	$state<Value>(value: Value): Value {
		return value;
	},
	$derived<Value>(value: Value): Value {
		return value;
	}
});

const { OrganizationDirectoryController } = await import('../../../src/routes/organization/organization-directory-controller.svelte');

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		userID: '',
		handle: '',
		name: '',
		email: '',
		...overrides
	};
}

describe('organization directory controller', () => {
	test('exposes the root and selected organization subtree in priority order', () => {
		const controller = new OrganizationDirectoryController('/admin/api', organizationDirectoryText.ko, adminText.ko);
		controller.groups = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'sales', name: '세일즈' }
		];
		controller.records = [
			userRecord({ userID: 'lead', groupID: 'product' }),
			userRecord({ userID: 'engineer', groupID: 'engineering' }),
			userRecord({ userID: 'sales', groupID: 'sales' })
		];

		expect(controller.organizationSections.map((section) => section.id)).toEqual(['', 'sales', 'product', 'engineering']);
		expect(controller.organizationTree.nodes.find((node) => node.id === 'product')?.memberCount).toBe(2);

		controller.groupID = 'product';
		expect(controller.organizationSections.map((section) => section.id)).toEqual(['product', 'engineering']);
		expect(controller.selectedOrganizationName).toBe('프로덕트 본부');
	});

	test('labels and counts unassigned members separately from all organizations', () => {
		const controller = new OrganizationDirectoryController('/admin/api', organizationDirectoryText.ko, adminText.ko);
		controller.groups = [{ id: 'product', name: '프로덕트 본부' }];
		controller.records = [
			userRecord({ userID: 'assigned', groupID: 'product' }),
			userRecord({ userID: 'unassigned' })
		];
		controller.groupID = unassignedGroupID;

		expect(controller.selectedOrganizationName).toBe('팀 미지정');
		expect(controller.organizationSections.length).toBe(1);
		expect(controller.organizationSections[0]).toMatchObject({ name: '팀 미지정', memberCount: 1 });
		expect(controller.organizationSections.flatMap((section) => section.records).map((record) => record.userID)).toEqual(['unassigned']);
	});
});
