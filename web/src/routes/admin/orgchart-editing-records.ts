import type { OrgGroup, UserRecord } from '../orgchart/orgchart-types';

export function copyUserRecord(record: UserRecord): UserRecord {
	return {
		...record,
		groupIDs: [...(record.groupIDs ?? [])]
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
	const primaryGroupID = editingRecord.primaryGroupID && availableGroupIDs.has(editingRecord.primaryGroupID) ? editingRecord.primaryGroupID : responseRecord.primaryGroupID ?? '';
	return {
		...editingRecord,
		group: primaryGroupID,
		primaryGroupID,
		groupIDs: primaryGroupID ? [primaryGroupID] : []
	};
}
