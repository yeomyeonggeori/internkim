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
		if (filters.groupID === unassignedGroupID && recordGroupID(record)) return false;
		if (filters.groupID && filters.groupID !== unassignedGroupID && recordGroupID(record) !== filters.groupID) return false;
		if (query && !searchableText(record).includes(query)) return false;
		return true;
	});
}

export function organizationFilterOptions(records: UserRecord[], groups: OrgGroup[]): OrganizationDirectoryOptions {
	const visibleGroupIDs = new Set(records.map(recordGroupID));
	return {
		groups: groups.filter((group) => visibleGroupIDs.has(group.id)),
		hasUnassigned: records.some((record) => recordGroupID(record) === '')
	};
}

function recordGroupID(record: UserRecord): string {
	return (record.groupID ?? '').trim();
}

function searchableText(record: UserRecord): string {
	return normalizedSearch([record.name, record.jobTitle].filter(Boolean).join(' '));
}

function normalizedSearch(value: string): string {
	return value.trim().toLowerCase();
}
