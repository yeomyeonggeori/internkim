import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/routes/admin/admin-types';
import {
	beginOrgchartProfileEdit,
	clearOrgchartProfileSaving,
	hasUnsavedOrgchartProfileEdits,
	markOrgchartProfileSaving,
	orgchartProfileSavePayload,
	orgchartProfileSnapshots,
	removeOrgchartProfileEdit
} from '../../../src/routes/orgchart/orgchart-profile-edit-controller';

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		userID: '',
		handle: '',
		name: '',
		email: '',
		role: 'member',
		...overrides
	};
}

describe('orgchart profile edit controller', () => {
	test('tracks a copied editing draft against original profile snapshots', () => {
		const record = userRecord({
			userID: 'dabin',
			email: 'dabin@example.com',
			jobTitle: '프론트엔드 개발자',
			primaryGroupID: 'product',
			groupIDs: ['product']
		});
		const originalProfiles = orgchartProfileSnapshots([record]);
		const editingRecords = beginOrgchartProfileEdit({}, record);

		editingRecords.dabin.jobTitle = '제품 개발자';

		expect(record.jobTitle).toBe('프론트엔드 개발자');
		expect(hasUnsavedOrgchartProfileEdits(editingRecords, originalProfiles)).toBe(true);
		expect(orgchartProfileSavePayload(editingRecords.dabin)).toEqual({
			userID: 'dabin',
			email: 'dabin@example.com',
			jobTitle: '제품 개발자',
			group: 'product',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: ''
		});
	});

	test('removes editing and saving entries by user id', () => {
		const editingRecords = {
			dabin: userRecord({ userID: 'dabin' }),
			junho: userRecord({ userID: 'junho' })
		};
		const savingRecords = markOrgchartProfileSaving({}, 'dabin');

		expect(removeOrgchartProfileEdit(editingRecords, 'dabin')).toEqual({
			junho: editingRecords.junho
		});
		expect(clearOrgchartProfileSaving(savingRecords, 'dabin')).toEqual({});
	});
});
