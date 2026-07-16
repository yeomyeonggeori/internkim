type OrgchartGroupRecord = {
	primaryGroupID?: string;
	group?: string;
	groupIDs?: string[];
};

export type OrgchartGroupMembership = {
	primaryGroupID: string;
	groupIDs: string[];
};

export function orgchartGroupMembership(record: OrgchartGroupRecord): OrgchartGroupMembership {
	const explicitPrimaryGroupID = normalizeOrgchartGroupID(record.primaryGroupID);
	const legacyPrimaryGroupID = normalizeOrgchartGroupID(record.group);
	const normalizedGroupIDs = normalizeOrgchartGroupIDs(record.groupIDs ?? []);
	const primaryGroupID = explicitPrimaryGroupID || legacyPrimaryGroupID || normalizedGroupIDs[0] || '';
	const replacedPrimaryGroupID = explicitPrimaryGroupID && legacyPrimaryGroupID !== explicitPrimaryGroupID ? legacyPrimaryGroupID : '';
	const secondaryGroupIDs = normalizedGroupIDs.filter((groupID) => groupID !== primaryGroupID && groupID !== replacedPrimaryGroupID);
	return {
		primaryGroupID,
		groupIDs: primaryGroupID ? [primaryGroupID, ...secondaryGroupIDs] : secondaryGroupIDs
	};
}

export function replaceOrgchartPrimaryGroup(record: OrgchartGroupRecord, nextPrimaryGroupID: string): OrgchartGroupMembership {
	const membership = orgchartGroupMembership(record);
	const primaryGroupID = normalizeOrgchartGroupID(nextPrimaryGroupID);
	if (!primaryGroupID) return { primaryGroupID: '', groupIDs: [] };
	const secondaryGroupIDs = membership.groupIDs.filter((groupID) => groupID !== membership.primaryGroupID && groupID !== primaryGroupID);
	return {
		primaryGroupID,
		groupIDs: [primaryGroupID, ...secondaryGroupIDs]
	};
}

function normalizeOrgchartGroupIDs(groupIDs: string[]): string[] {
	return [...new Set(groupIDs.map(normalizeOrgchartGroupID).filter(Boolean))];
}

function normalizeOrgchartGroupID(groupID: string | undefined): string {
	return (groupID ?? '').trim();
}
