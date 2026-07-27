import { organizationGroupMembership } from '../../lib/organization/group-membership';
import type { UserRecord } from '../../lib/organization/types';
import type { OrgProfileUpdate } from './admin-api';

export type OrgProfileSnapshot = {
	jobTitle: string;
	primaryGroupID: string;
	groupIDs: string[];
	supervisorID: string;
};

export function normalizeOrgProfileRecord(record: UserRecord): UserRecord {
	const membership = organizationGroupMembership(record);
	return {
		...record,
		name: record.name ?? '',
		jobTitle: record.jobTitle?.trim() ?? '',
		group: membership.primaryGroupID,
		primaryGroupID: membership.primaryGroupID,
		groupIDs: membership.groupIDs,
		supervisorID: record.supervisorID ?? ''
	};
}

export function orgProfileSnapshot(record: UserRecord): OrgProfileSnapshot {
	const membership = organizationGroupMembership(record);
	return {
		jobTitle: record.jobTitle?.trim() ?? '',
		primaryGroupID: membership.primaryGroupID,
		groupIDs: membership.groupIDs,
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
