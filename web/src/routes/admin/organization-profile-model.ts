import type { UserRecord } from '../../lib/organization/types';
import type { OrgProfileUpdate } from './admin-api';

export type OrgProfileSnapshot = {
	jobTitle: string;
	groupID: string;
	supervisorID: string;
};

export function normalizeOrgProfileRecord(record: UserRecord): UserRecord {
	return {
		...record,
		name: record.name ?? '',
		jobTitle: record.jobTitle?.trim() ?? '',
		groupID: record.groupID ?? '',
		supervisorID: record.supervisorID ?? ''
	};
}

export function orgProfileSnapshot(record: UserRecord): OrgProfileSnapshot {
	return {
		jobTitle: record.jobTitle?.trim() ?? '',
		groupID: record.groupID ?? '',
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
		groupID: snapshot.groupID,
		supervisorID: snapshot.supervisorID
	};
}
