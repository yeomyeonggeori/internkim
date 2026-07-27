import { organizationGroupMembership } from '../../lib/organization/group-membership';
import type { OrgGroup, UserRecord } from '../../lib/organization/types';

export type OrganizationDirectoryFilters = {
	query: string;
	groupID: string;
};

export type OrganizationDirectoryOptions = {
	groups: OrgGroup[];
	hasUnassigned: boolean;
};

export const unassignedGroupID = '__unassigned__';

export function filterOrganizationRecords(records: UserRecord[], filters: OrganizationDirectoryFilters): UserRecord[] {
	const query = normalizedSearch(filters.query);
	return records.filter((record) => {
		if (filters.groupID === unassignedGroupID && recordPrimaryGroupID(record)) return false;
		if (filters.groupID && filters.groupID !== unassignedGroupID && !recordGroupIDs(record).includes(filters.groupID)) return false;
		if (query && !searchableText(record).includes(query)) return false;
		return true;
	});
}

export function organizationFilterOptions(records: UserRecord[], groups: OrgGroup[]): OrganizationDirectoryOptions {
	const visibleGroupIDs = new Set(records.flatMap(recordGroupIDs));
	return {
		groups: groups.filter((group) => visibleGroupIDs.has(group.id)),
		hasUnassigned: records.some((record) => recordPrimaryGroupID(record) === '')
	};
}

function recordGroupIDs(record: UserRecord): string[] {
	return organizationGroupMembership(record).groupIDs;
}

function recordPrimaryGroupID(record: UserRecord): string {
	return organizationGroupMembership(record).primaryGroupID;
}

function searchableText(record: UserRecord): string {
	return normalizedSearch([record.name, record.email, record.handle, record.jobTitle].filter(Boolean).join(' '));
}

function normalizedSearch(value: string): string {
	return value.trim().toLowerCase();
}
