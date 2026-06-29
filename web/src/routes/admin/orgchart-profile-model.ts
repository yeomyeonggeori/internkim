import type { OrgchartEmploymentStatus, UserRecord } from './admin-types';
import type { OrgProfileUpdate } from './admin-api';

export type OrgProfileSnapshot = {
	jobTitle: string;
	positionLevel?: number;
	primaryGroupID: string;
	groupIDs: string[];
	supervisorID: string;
	projectIDs: string[];
	teamRole: string;
	employmentStatus: OrgchartEmploymentStatus;
	isOrgchartVisible: boolean;
};

export const orgchartEmploymentStatuses: OrgchartEmploymentStatus[] = ['active', 'leave', 'resigned'];

export function normalizeOrgProfileRecord(record: UserRecord): UserRecord {
	const primaryGroupID = primaryGroupIDOf(record);
	return {
		...record,
		name: record.name ?? '',
		jobTitle: record.jobTitle?.trim() ?? '',
		group: primaryGroupID,
		positionLevel: normalizePositionLevel(record.positionLevel),
		primaryGroupID,
		groupIDs: normalizeGroupIDs(record.groupIDs ?? groupIDsFromLegacyGroup(record.group), primaryGroupID),
		supervisorID: record.supervisorID ?? '',
		projectIDs: normalizeStringList(record.projectIDs),
		teamRole: record.teamRole?.trim() ?? '',
		employmentStatus: record.employmentStatus ?? 'active',
		isOrgchartVisible: record.isOrgchartVisible ?? true
	};
}

export function orgProfileSnapshot(record: UserRecord): OrgProfileSnapshot {
	const primaryGroupID = primaryGroupIDOf(record);
	return {
		jobTitle: record.jobTitle?.trim() ?? '',
		positionLevel: normalizePositionLevel(record.positionLevel),
		primaryGroupID,
		groupIDs: normalizeGroupIDs(record.groupIDs ?? groupIDsFromLegacyGroup(record.group), primaryGroupID),
		supervisorID: record.supervisorID ?? '',
		projectIDs: normalizeStringList(record.projectIDs),
		teamRole: record.teamRole?.trim() ?? '',
		employmentStatus: record.employmentStatus ?? 'active',
		isOrgchartVisible: record.isOrgchartVisible ?? true
	};
}

export function isOrgProfileChanged(record: UserRecord, original: OrgProfileSnapshot | undefined): boolean {
	if (!original) return false;
	return JSON.stringify(orgProfileSnapshot(record)) !== JSON.stringify(original);
}

export function isPositionLevelInvalid(record: UserRecord): boolean {
	if (record.positionLevel === undefined) return false;
	return !Number.isInteger(record.positionLevel) || record.positionLevel < 1;
}

export function orgProfileUpdate(record: UserRecord): OrgProfileUpdate {
	const snapshot = orgProfileSnapshot(record);
	return {
		userID: record.userID,
		email: record.email,
		jobTitle: snapshot.jobTitle,
		group: snapshot.primaryGroupID,
		positionLevel: snapshot.positionLevel,
		primaryGroupID: snapshot.primaryGroupID,
		groupIDs: snapshot.groupIDs,
		supervisorID: snapshot.supervisorID,
		projectIDs: snapshot.projectIDs,
		teamRole: snapshot.teamRole,
		employmentStatus: snapshot.employmentStatus,
		isOrgchartVisible: snapshot.isOrgchartVisible
	};
}

function primaryGroupIDOf(record: UserRecord): string {
	return (record.primaryGroupID ?? record.group ?? '').trim();
}

function groupIDsFromLegacyGroup(groupID: string | undefined): string[] {
	return groupID ? [groupID] : [];
}

function normalizeGroupIDs(groupIDs: string[], primaryGroupID: string): string[] {
	const normalizedGroupIDs = normalizeStringList(primaryGroupID ? [primaryGroupID, ...groupIDs] : groupIDs);
	return normalizedGroupIDs;
}

function normalizeStringList(values: string[] | undefined): string[] {
	return [...new Set((values ?? []).map((value) => value.trim()).filter(Boolean))];
}

function normalizePositionLevel(positionLevel: number | undefined): number | undefined {
	if (positionLevel === undefined || Number.isNaN(positionLevel)) return undefined;
	return positionLevel;
}
