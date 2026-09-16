import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/lib/organization/types';
import {
	beginOrganizationProfileEdit,
	clearOrganizationProfileSaving,
	hasUnsavedOrganizationProfileEdits,
	markOrganizationProfileSaving,
	normalizedOrganizationRecords,
	organizationProfileSavePayload,
	organizationProfileSnapshots,
	removeOrganizationProfileEdit
} from '../../../src/routes/organization/organization-profile-edit-controller';

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		memberID: '',
		handle: '',
		name: '',
		email: '',
		...overrides
	};
}

describe('organization profile edit controller', () => {
	test('preserves the organization membership while normalizing directory records', () => {
		const [record] = normalizedOrganizationRecords([
			userRecord({
				memberID: 'ceo',
				groupID: 'leadership'
			})
		]) ?? [];

		expect(record.groupID).toBe('leadership');
	});

	test('tracks a copied editing draft against original profile snapshots', () => {
		const record = userRecord({
			memberID: 'dabin',
			email: 'dabin@example.com',
			jobTitle: '프론트엔드 개발자',
			groupID: 'product'
		});
		const originalProfiles = organizationProfileSnapshots([record]);
		const editingRecords = beginOrganizationProfileEdit({}, record);

		editingRecords.dabin.jobTitle = '제품 개발자';

		expect(record.jobTitle).toBe('프론트엔드 개발자');
		expect(hasUnsavedOrganizationProfileEdits(editingRecords, originalProfiles)).toBe(true);
		expect(organizationProfileSavePayload(editingRecords.dabin)).toEqual({
			memberID: 'dabin',
			jobTitle: '제품 개발자',
			groupID: 'product',
			hireDate: '',
			phoneNumber: '',
			supervisorID: ''
		});
	});

	test('sends the clearance only when the draft changed it', () => {
		const record = userRecord({ memberID: 'dabin', email: 'dabin@example.com', clearance: 1 });
		const originalProfiles = organizationProfileSnapshots([record]);
		const editingRecords = beginOrganizationProfileEdit({}, record);

		editingRecords.dabin.jobTitle = '제품 개발자';
		expect(hasUnsavedOrganizationProfileEdits(editingRecords, originalProfiles)).toBe(true);
		expect(organizationProfileSavePayload(editingRecords.dabin, originalProfiles)).not.toHaveProperty('clearance');

		editingRecords.dabin.clearance = 2;
		expect(organizationProfileSavePayload(editingRecords.dabin, originalProfiles)).toMatchObject({ memberID: 'dabin', clearance: 2 });
	});

	test('a clearance change alone counts as an unsaved edit', () => {
		const record = userRecord({ memberID: 'dabin', email: 'dabin@example.com', clearance: 1 });
		const originalProfiles = organizationProfileSnapshots([record]);
		const editingRecords = beginOrganizationProfileEdit({}, record);

		expect(hasUnsavedOrganizationProfileEdits(editingRecords, originalProfiles)).toBe(false);
		editingRecords.dabin.clearance = 3;
		expect(hasUnsavedOrganizationProfileEdits(editingRecords, originalProfiles)).toBe(true);
	});

	test('removes editing and saving entries by user id', () => {
		const editingRecords = {
			dabin: userRecord({ memberID: 'dabin' }),
			junho: userRecord({ memberID: 'junho' })
		};
		const savingRecords = markOrganizationProfileSaving({}, 'dabin');

		expect(removeOrganizationProfileEdit(editingRecords, 'dabin')).toEqual({
			junho: editingRecords.junho
		});
		expect(clearOrganizationProfileSaving(savingRecords, 'dabin')).toEqual({});
	});
});
