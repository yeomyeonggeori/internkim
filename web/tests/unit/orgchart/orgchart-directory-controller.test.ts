import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/lib/orgchart/types';
import { adminText } from '../../../src/routes/admin/text';
import { unassignedGroupID } from '../../../src/routes/orgchart/orgchart-directory-model';
import { orgchartDirectoryText } from '../../../src/routes/orgchart/text';

Object.assign(globalThis, {
	$state<Value>(value: Value): Value {
		return value;
	},
	$derived<Value>(value: Value): Value {
		return value;
	}
});

const { OrgchartDirectoryController } = await import('../../../src/routes/orgchart/orgchart-directory-controller.svelte');

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		userID: '',
		handle: '',
		name: '',
		email: '',
		...overrides
	};
}

describe('orgchart directory controller', () => {
	test('exposes the root and selected organization subtree', () => {
		const controller = new OrgchartDirectoryController('/admin/api', orgchartDirectoryText.ko, adminText.ko);
		controller.groups = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'sales', name: '세일즈' }
		];
		controller.records = [
			userRecord({ userID: 'lead', primaryGroupID: 'product' }),
			userRecord({ userID: 'engineer', primaryGroupID: 'engineering' }),
			userRecord({ userID: 'sales', primaryGroupID: 'sales' })
		];

		expect(controller.organizationSections.map((section) => section.id)).toEqual(['', 'product', 'engineering', 'sales']);
		expect(controller.organizationTree.nodes.find((node) => node.id === 'product')?.memberCount).toBe(2);

		controller.groupID = 'product';
		expect(controller.organizationSections.map((section) => section.id)).toEqual(['product', 'engineering']);
		expect(controller.selectedOrganizationName).toBe('프로덕트 본부');
	});

	test('labels and counts unassigned members separately from all organizations', () => {
		const controller = new OrgchartDirectoryController('/admin/api', orgchartDirectoryText.ko, adminText.ko);
		controller.groups = [{ id: 'product', name: '프로덕트 본부' }];
		controller.records = [
			userRecord({ userID: 'assigned', primaryGroupID: 'product' }),
			userRecord({ userID: 'unassigned' })
		];
		controller.groupID = unassignedGroupID;

		expect(controller.selectedOrganizationName).toBe('팀 미지정');
		expect(controller.organizationSections.length).toBe(1);
		expect(controller.organizationSections[0]).toMatchObject({ name: '팀 미지정', memberCount: 1 });
		expect(controller.organizationSections.flatMap((section) => section.records).map((record) => record.userID)).toEqual(['unassigned']);
	});
});
