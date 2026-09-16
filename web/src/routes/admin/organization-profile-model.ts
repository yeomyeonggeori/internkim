import type { UserRecord } from '../../lib/organization/types';
import type { OrgProfileUpdate } from '$lib/organization/types';

export type OrgProfileSnapshot = {
	jobTitle: string;
	groupID: string;
	hireDate: string;
	phoneNumber: string;
	supervisorID: string;
	clearance?: number;
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
		supervisorID: record.supervisorID ?? '',
		clearance: record.clearance
	};
}

export function isOrgProfileChanged(record: UserRecord, original: OrgProfileSnapshot | undefined): boolean {
	if (!original) return false;
	return JSON.stringify(orgProfileSnapshot(record)) !== JSON.stringify(original);
}

export function orgProfileUpdate(record: UserRecord): OrgProfileUpdate {
	const snapshot = orgProfileSnapshot(record);
	return {
		memberID: record.memberID,
		jobTitle: snapshot.jobTitle,
		groupID: snapshot.groupID,
		hireDate: snapshot.hireDate,
		phoneNumber: snapshot.phoneNumber,
		supervisorID: snapshot.supervisorID
	};
}
