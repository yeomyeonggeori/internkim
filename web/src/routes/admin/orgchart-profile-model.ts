import type { UserRecord } from '../../lib/orgchart/types';
import type { OrgProfileUpdate } from './admin-api';

export type OrgProfileSnapshot = {
	jobTitle: string;
	primaryGroupID: string;
	groupIDs: string[];
	supervisorID: string;
};

export function normalizeOrgProfileRecord(record: UserRecord): UserRecord {
	const primaryGroupID = primaryGroupIDOf(record);
	return {
		...record,
		name: record.name ?? '',
		jobTitle: record.jobTitle?.trim() ?? '',
		group: primaryGroupID,
		primaryGroupID,
		groupIDs: groupIDsFromPrimary(primaryGroupID),
		supervisorID: record.supervisorID ?? ''
	};
}

export function orgProfileSnapshot(record: UserRecord): OrgProfileSnapshot {
	const primaryGroupID = primaryGroupIDOf(record);
	return {
		jobTitle: record.jobTitle?.trim() ?? '',
		primaryGroupID,
		groupIDs: groupIDsFromPrimary(primaryGroupID),
		supervisorID: record.supervisorID ?? ''
	};
}

export function isOrgProfileChanged(record: UserRecord, original: OrgProfileSnapshot | undefined): boolean {
	if (!original) return false;
	return JSON.stringify(orgProfileSnapshot(record)) !== JSON.stringify(original);
}

export function orgProfileUpdate(record: UserRecord): OrgProfileUpdate {
	const snapshot = orgProfileSnapshot(record);
	return {
		userID: record.userID,
		email: record.email,
		jobTitle: snapshot.jobTitle,
		group: snapshot.primaryGroupID,
		primaryGroupID: snapshot.primaryGroupID,
		groupIDs: snapshot.groupIDs,
		supervisorID: snapshot.supervisorID
	};
}

function primaryGroupIDOf(record: UserRecord): string {
	return record.primaryGroupID?.trim() || record.group?.trim() || firstGroupID(record.groupIDs);
}

function groupIDsFromPrimary(primaryGroupID: string): string[] {
	return primaryGroupID ? [primaryGroupID] : [];
}

function firstGroupID(groupIDs: string[] | undefined): string {
	return groupIDs?.find((groupID) => groupID.trim())?.trim() ?? '';
}
