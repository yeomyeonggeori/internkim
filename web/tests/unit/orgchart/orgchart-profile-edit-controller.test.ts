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
	test('preserves every organization membership while normalizing directory records', () => {
		const [record] = normalizedOrganizationRecords([
			userRecord({
				userID: 'ceo',
				primaryGroupID: 'leadership',
				groupIDs: ['leadership', 'product', 'product']
			})
		]) ?? [];

		expect(record.groupIDs).toEqual(['leadership', 'product']);
	});

	test('tracks a copied editing draft against original profile snapshots', () => {
		const record = userRecord({
			userID: 'dabin',
			email: 'dabin@example.com',
			jobTitle: '프론트엔드 개발자',
			primaryGroupID: 'product',
			groupIDs: ['product']
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
			group: 'product',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: ''
		});
	});

	test('keeps secondary organizations when only the job title changes', () => {
		const record = userRecord({
			userID: 'dabin',
			email: 'dabin@example.com',
			jobTitle: '프론트엔드 개발자',
			primaryGroupID: 'product',
			group: 'product',
			groupIDs: ['product', 'platform']
		});
		const editingRecord = beginOrganizationProfileEdit({}, record).dabin;

		editingRecord.jobTitle = '제품 개발자';

		expect(organizationProfileSavePayload(editingRecord).groupIDs).toEqual(['product', 'platform']);
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
