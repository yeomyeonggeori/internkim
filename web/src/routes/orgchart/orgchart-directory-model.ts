import { orgchartGroupMembership } from '../../lib/orgchart/group-membership';
import type { OrgGroup, UserRecord } from '../../lib/orgchart/types';

export type OrgchartDirectoryFilters = {
	query: string;
	groupID: string;
};

export type OrgchartDirectoryOptions = {
	groups: OrgGroup[];
	hasUnassigned: boolean;
};

export const unassignedGroupID = '__unassigned__';

export function filterOrgchartRecords(records: UserRecord[], filters: OrgchartDirectoryFilters): UserRecord[] {
	const query = normalizedSearch(filters.query);
	return records.filter((record) => {
		if (filters.groupID === unassignedGroupID && recordPrimaryGroupID(record)) return false;
		if (filters.groupID && filters.groupID !== unassignedGroupID && !recordGroupIDs(record).includes(filters.groupID)) return false;
		if (query && !searchableText(record).includes(query)) return false;
		return true;
	});
}

export function orgchartFilterOptions(records: UserRecord[], groups: OrgGroup[]): OrgchartDirectoryOptions {
	const visibleGroupIDs = new Set(records.flatMap(recordGroupIDs));
	return {
		groups: groups.filter((group) => visibleGroupIDs.has(group.id)),
		hasUnassigned: records.some((record) => recordPrimaryGroupID(record) === '')
	};
}

function recordGroupIDs(record: UserRecord): string[] {
	return orgchartGroupMembership(record).groupIDs;
}

function recordPrimaryGroupID(record: UserRecord): string {
	return orgchartGroupMembership(record).primaryGroupID;
}

function searchableText(record: UserRecord): string {
	return normalizedSearch([record.name, record.email, record.handle, record.jobTitle].filter(Boolean).join(' '));
}

function normalizedSearch(value: string): string {
	return value.trim().toLowerCase();
}
