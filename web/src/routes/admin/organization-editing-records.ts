import type { OrgGroup, UserRecord } from '../../lib/organization/types';

export function copyUserRecord(record: UserRecord): UserRecord {
	return { ...record };
}

export function reconcileEditingRecords(editingRecords: Record<string, UserRecord>, responseRecords: UserRecord[], availableGroups: OrgGroup[]): Record<string, UserRecord> {
	const availableGroupIDs = new Set(availableGroups.map((group) => group.id));
	const responseRecordsByUserID = new Map(responseRecords.map((record) => [record.userID, record]));
	return Object.fromEntries(
		Object.entries(editingRecords).map(([userID, editingRecord]) => {
			const responseRecord = responseRecordsByUserID.get(userID);
			if (!responseRecord) return [userID, editingRecord];
			return [userID, reconcileEditingRecordGroup(editingRecord, responseRecord, availableGroupIDs)];
		})
	);
}

function reconcileEditingRecordGroup(editingRecord: UserRecord, responseRecord: UserRecord, availableGroupIDs: Set<string>): UserRecord {
	const editingGroupID = editingRecord.groupID ?? '';
	const groupID = availableGroupIDs.has(editingGroupID) ? editingGroupID : responseRecord.groupID ?? '';
	return { ...editingRecord, groupID };
}
