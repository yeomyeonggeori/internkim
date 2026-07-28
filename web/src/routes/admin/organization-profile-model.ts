import type { UserRecord } from '../../lib/organization/types';
import type { OrgProfileUpdate } from './admin-api';

export type OrgProfileSnapshot = {
	jobTitle: string;
	groupID: string;
	hireDate: string;
	phoneNumber: string;
	supervisorID: string;
};

export function normalizeOrgProfileRecord(record: UserRecord): UserRecord {
	return {
		...record,
		name: record.name ?? '',
		jobTitle: record.jobTitle?.trim() ?? '',
		groupID: record.groupID ?? '',
		hireDate: record.hireDate ?? '',
		phoneNumber: record.phoneNumber ?? '',
		supervisorID: record.supervisorID ?? ''
	};
}

export function orgProfileSnapshot(record: UserRecord): OrgProfileSnapshot {
	return {
		jobTitle: record.jobTitle?.trim() ?? '',
		groupID: record.groupID ?? '',
		hireDate: record.hireDate?.trim() ?? '',
		phoneNumber: record.phoneNumber?.trim() ?? '',
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
		hireDate: snapshot.hireDate,
		phoneNumber: snapshot.phoneNumber,
		supervisorID: snapshot.supervisorID
	};
}
