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
		userID: '',
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
				userID: 'ceo',
				groupID: 'leadership'
			})
		]) ?? [];

		expect(record.groupID).toBe('leadership');
	});

	test('tracks a copied editing draft against original profile snapshots', () => {
		const record = userRecord({
			userID: 'dabin',
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
			userID: 'dabin',
			email: 'dabin@example.com',
			jobTitle: '제품 개발자',
			groupID: 'product',
			phoneNumber: '',
			supervisorID: ''
		});
	});

	test('removes editing and saving entries by user id', () => {
		const editingRecords = {
			dabin: userRecord({ userID: 'dabin' }),
			junho: userRecord({ userID: 'junho' })
		};
		const savingRecords = markOrganizationProfileSaving({}, 'dabin');

		expect(removeOrganizationProfileEdit(editingRecords, 'dabin')).toEqual({
			junho: editingRecords.junho
		});
		expect(clearOrganizationProfileSaving(savingRecords, 'dabin')).toEqual({});
	});
});
