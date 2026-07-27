import type { OrgGroup } from '../../lib/organization/types';

export type OrganizationGroupSavePlan = {
	groupID: string;
	groups: OrgGroup[];
	shouldPersist: boolean;
};

export function organizationGroupSavePlan(groups: OrgGroup[], name: string, parentID: string, createGroupID: () => string): OrganizationGroupSavePlan {
	const trimmedName = name.trim();
	if (!trimmedName) return { groupID: '', groups, shouldPersist: false };

	const existingGroupID = groupIDByName(groups, trimmedName);
	if (existingGroupID) return { groupID: existingGroupID, groups, shouldPersist: false };

	const group = { id: createGroupID(), name: trimmedName, ...(parentID ? { parentID } : {}) };
	return {
		groupID: group.id,
		groups: [...groups, group],
		shouldPersist: true
	};
}

function groupIDByName(groups: OrgGroup[], name: string): string {
	const normalizedName = normalizedGroupName(name);
	return groups.find((group) => normalizedGroupName(group.name) === normalizedName)?.id ?? '';
}

function normalizedGroupName(name: string): string {
	return name.trim().toLowerCase();
}
