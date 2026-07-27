type OrganizationGroupRecord = {
	primaryGroupID?: string;
	group?: string;
	groupIDs?: string[];
};

export type OrganizationGroupMembership = {
	primaryGroupID: string;
	groupIDs: string[];
};

export function organizationGroupMembership(record: OrganizationGroupRecord): OrganizationGroupMembership {
	const explicitPrimaryGroupID = normalizeOrganizationGroupID(record.primaryGroupID);
	const legacyPrimaryGroupID = normalizeOrganizationGroupID(record.group);
	const normalizedGroupIDs = normalizeOrganizationGroupIDs(record.groupIDs ?? []);
	const primaryGroupID = explicitPrimaryGroupID || legacyPrimaryGroupID || normalizedGroupIDs[0] || '';
	const replacedPrimaryGroupID = explicitPrimaryGroupID && legacyPrimaryGroupID !== explicitPrimaryGroupID ? legacyPrimaryGroupID : '';
	const secondaryGroupIDs = normalizedGroupIDs.filter((groupID) => groupID !== primaryGroupID && groupID !== replacedPrimaryGroupID);
	return {
		primaryGroupID,
		groupIDs: primaryGroupID ? [primaryGroupID, ...secondaryGroupIDs] : secondaryGroupIDs
	};
}

export function replaceOrganizationPrimaryGroup(record: OrganizationGroupRecord, nextPrimaryGroupID: string): OrganizationGroupMembership {
	const membership = organizationGroupMembership(record);
	const primaryGroupID = normalizeOrganizationGroupID(nextPrimaryGroupID);
	if (!primaryGroupID) return { primaryGroupID: '', groupIDs: [] };
	const secondaryGroupIDs = membership.groupIDs.filter((groupID) => groupID !== membership.primaryGroupID && groupID !== primaryGroupID);
	return {
		primaryGroupID,
		groupIDs: [primaryGroupID, ...secondaryGroupIDs]
	};
}

function normalizeOrganizationGroupIDs(groupIDs: string[]): string[] {
	return [...new Set(groupIDs.map(normalizeOrganizationGroupID).filter(Boolean))];
}

function normalizeOrganizationGroupID(groupID: string | undefined): string {
	return (groupID ?? '').trim();
}
