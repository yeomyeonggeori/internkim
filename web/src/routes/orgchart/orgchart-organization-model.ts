import { compareOrgchartPeople } from '$lib/orgchart/person-order';
import type { OrgGroup, UserRecord } from '../admin/admin-types';
import { unassignedGroupID } from './orgchart-directory-model';

export type OrgchartOrganizationMemberNode = {
	record: UserRecord;
	children: OrgchartOrganizationMemberNode[];
};

export type OrgchartOrganizationSection = {
	id: string;
	name: string;
	leader: UserRecord;
	memberNodes: OrgchartOrganizationMemberNode[];
	memberCount: number;
	isUnassigned: boolean;
};

type OrganizationDefinition = {
	id: string;
	name: string;
	isUnassigned: boolean;
};

export function orgchartOrganizationSections(records: UserRecord[], groups: OrgGroup[], unassignedName: string): OrgchartOrganizationSection[] {
	const recordsByGroupID = recordsByPrimaryGroupID(records);
	return organizationDefinitions(groups, recordsByGroupID, unassignedName).flatMap((organization) => {
		const organizationRecords = sortedRecords(recordsByGroupID.get(organization.id) ?? []);
		if (organizationRecords.length === 0) return [];
		const treeRoots = organizationTreeRoots(organizationRecords);
		const leaderNode = treeRoots[0] ?? { record: organizationRecords[0], children: [] };
		return [
			{
				id: organization.id,
				name: organization.name,
				leader: leaderNode.record,
				memberNodes: [...leaderNode.children, ...treeRoots.slice(1)],
				memberCount: organizationRecords.length,
				isUnassigned: organization.isUnassigned
			}
		];
	});
}

function organizationDefinitions(groups: OrgGroup[], recordsByGroupID: Map<string, UserRecord[]>, unassignedName: string): OrganizationDefinition[] {
	const knownGroupIDs = new Set(groups.map((group) => group.id));
	const unknownGroups = Array.from(recordsByGroupID.keys())
		.filter((groupID) => groupID !== unassignedGroupID && !knownGroupIDs.has(groupID))
		.sort((first, second) => first.localeCompare(second))
		.map((groupID) => ({ id: groupID, name: groupID, isUnassigned: false }));
	const unassignedDefinition = recordsByGroupID.has(unassignedGroupID) ? [{ id: unassignedGroupID, name: unassignedName, isUnassigned: true }] : [];
	return [
		...groups.map((group) => ({ id: group.id, name: group.name, isUnassigned: false })),
		...unknownGroups,
		...unassignedDefinition
	];
}

function recordsByPrimaryGroupID(records: UserRecord[]): Map<string, UserRecord[]> {
	const recordsByGroupID = new Map<string, UserRecord[]>();
	for (const record of records) {
		const groupID = primaryGroupID(record) || unassignedGroupID;
		recordsByGroupID.set(groupID, [...(recordsByGroupID.get(groupID) ?? []), record]);
	}
	return recordsByGroupID;
}

function organizationTreeRoots(records: UserRecord[]): OrgchartOrganizationMemberNode[] {
	const recordIDs = new Set(records.map((record) => record.userID));
	const nodesByID = new Map(records.map((record) => [record.userID, organizationMemberNode(record)]));
	const childIDs = new Set<string>();
	for (const record of records) {
		const supervisorID = normalizedID(record.supervisorID);
		const parentNode = supervisorID && recordIDs.has(supervisorID) ? nodesByID.get(supervisorID) : undefined;
		const childNode = nodesByID.get(record.userID);
		if (!parentNode || !childNode) continue;
		parentNode.children = sortedMemberNodes([...parentNode.children, childNode]);
		childIDs.add(record.userID);
	}
	return sortedMemberNodes(records.flatMap((record) => {
		const node = nodesByID.get(record.userID);
		return node && !childIDs.has(record.userID) ? [node] : [];
	}));
}

function organizationMemberNode(record: UserRecord): OrgchartOrganizationMemberNode {
	return {
		record,
		children: []
	};
}

function sortedMemberNodes(nodes: OrgchartOrganizationMemberNode[]): OrgchartOrganizationMemberNode[] {
	return [...nodes].sort((first, second) => compareRecords(first.record, second.record));
}

function sortedRecords(records: UserRecord[]): UserRecord[] {
	return [...records].sort(compareRecords);
}

function compareRecords(first: UserRecord, second: UserRecord): number {
	return compareOrgchartPeople(first, second);
}

function primaryGroupID(record: UserRecord): string {
	return (record.primaryGroupID ?? record.group ?? record.groupIDs?.[0] ?? '').trim();
}

function normalizedID(value: string | undefined): string {
	return (value ?? '').trim();
}
