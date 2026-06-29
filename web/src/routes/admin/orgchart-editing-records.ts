import type { OrgGroup, UserRecord } from './admin-types';

export function copyUserRecord(record: UserRecord): UserRecord {
	return {
		...record,
		groupIDs: [...(record.groupIDs ?? [])],
		projectIDs: [...(record.projectIDs ?? [])]
	};
}

export function reconcileEditingRecords(editingRecords: Record<string, UserRecord>, responseRecords: UserRecord[], availableGroups: OrgGroup[]): Record<string, UserRecord> {
	const availableGroupIDs = new Set(availableGroups.map((group) => group.id));
	const responseRecordsByUserID = new Map(responseRecords.map((record) => [record.userID, record]));
	return Object.fromEntries(
		Object.entries(editingRecords).map(([userID, editingRecord]) => {
			const responseRecord = responseRecordsByUserID.get(userID);
			if (!responseRecord) return [userID, editingRecord];
			return [userID, reconcileEditingRecordGroups(editingRecord, responseRecord, availableGroupIDs)];
		})
	);
}

function reconcileEditingRecordGroups(editingRecord: UserRecord, responseRecord: UserRecord, availableGroupIDs: Set<string>): UserRecord {
	const editableGroupIDs = (editingRecord.groupIDs ?? []).filter((groupID) => availableGroupIDs.has(groupID));
	const responseGroupIDs = (responseRecord.groupIDs ?? []).filter((groupID) => availableGroupIDs.has(groupID));
	const primaryGroupID = editingRecord.primaryGroupID && availableGroupIDs.has(editingRecord.primaryGroupID) ? editingRecord.primaryGroupID : responseRecord.primaryGroupID ?? '';
	const groupIDs = primaryGroupID ? [primaryGroupID, ...editableGroupIDs, ...responseGroupIDs] : [...editableGroupIDs, ...responseGroupIDs];
	return {
		...editingRecord,
		group: primaryGroupID,
		primaryGroupID,
		groupIDs: [...new Set(groupIDs)]
	};
}
