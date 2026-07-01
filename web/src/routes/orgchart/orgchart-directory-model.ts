import type { OrgGroup, UserRecord } from '../admin/admin-types';

export type OrgchartDirectoryFilters = {
	query: string;
	groupID: string;
};

export type OrgchartDirectoryOptions = {
	groups: OrgGroup[];
	hasUnassigned: boolean;
};

export type OrgchartPersonNode = {
	record: UserRecord;
	depth: number;
};

export type OrgchartTreeNode = OrgchartPersonNode & {
	children: OrgchartTreeNode[];
};

export type OrgchartTeamColumn = {
	id: string;
	name: string;
	records: UserRecord[];
	nodes: OrgchartPersonNode[];
	treeRoots: OrgchartTreeNode[];
	isUnassigned: boolean;
};

export type OrgchartCanvasModel = {
	roots: UserRecord[];
	columns: OrgchartTeamColumn[];
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

export function orgchartCanvasModel(records: UserRecord[], groups: OrgGroup[], unassignedName: string): OrgchartCanvasModel {
	const visibleRecords = sortedRecords(records);
	const visibleRecordIDs = new Set(visibleRecords.map((record) => record.userID));
	const roots = visibleRecords.filter((record) => {
		const supervisorID = normalizedID(record.supervisorID);
		return supervisorID === '' || !visibleRecordIDs.has(supervisorID) || hasSupervisorCycle(record, visibleRecords);
	});
	const recordDepths = recordDepthMap(visibleRecords, roots);
	const recordsByPrimaryGroupID = new Map<string, UserRecord[]>();
	for (const record of visibleRecords) {
		if (roots.some((root) => root.userID === record.userID)) continue;
		const groupID = recordPrimaryGroupID(record) || unassignedGroupID;
		recordsByPrimaryGroupID.set(groupID, [...(recordsByPrimaryGroupID.get(groupID) ?? []), record]);
	}
	const columns = groups.flatMap((group) => {
		const groupRecords = sortedColumnRecords(recordsByPrimaryGroupID.get(group.id) ?? [], recordDepths);
		if (groupRecords.length === 0) return [];
		return [teamColumn(group.id, group.name, groupRecords, false, recordDepths)];
	});
	const unassignedRecords = sortedColumnRecords(recordsByPrimaryGroupID.get(unassignedGroupID) ?? [], recordDepths);
	if (unassignedRecords.length === 0) return { roots, columns };
	return {
		roots,
		columns: [...columns, teamColumn(unassignedGroupID, unassignedName, unassignedRecords, true, recordDepths)]
	};
}

function teamColumn(id: string, name: string, records: UserRecord[], isUnassigned: boolean, recordDepths: Map<string, number>): OrgchartTeamColumn {
	return {
		id,
		name,
		records,
		nodes: records.map((record) => ({ record, depth: recordDepths.get(record.userID) ?? 1 })),
		treeRoots: teamTreeRoots(records, recordDepths),
		isUnassigned
	};
}

function teamTreeRoots(records: UserRecord[], recordDepths: Map<string, number>): OrgchartTreeNode[] {
	const recordIDs = new Set(records.map((record) => record.userID));
	const nodesByID = new Map(records.map((record) => [record.userID, treeNode(record, recordDepths)]));
	const childIDs = new Set<string>();
	for (const record of records) {
		const supervisorID = normalizedID(record.supervisorID);
		const parentNode = supervisorID && recordIDs.has(supervisorID) ? nodesByID.get(supervisorID) : undefined;
		const childNode = nodesByID.get(record.userID);
		if (!parentNode || !childNode) continue;
		parentNode.children = sortedTreeNodes([...parentNode.children, childNode]);
		childIDs.add(record.userID);
	}
	return sortedTreeNodes(records.flatMap((record) => {
		const node = nodesByID.get(record.userID);
		return node && !childIDs.has(record.userID) ? [node] : [];
	}));
}

function treeNode(record: UserRecord, recordDepths: Map<string, number>): OrgchartTreeNode {
	return {
		record,
		depth: recordDepths.get(record.userID) ?? 1,
		children: []
	};
}

function sortedTreeNodes(nodes: OrgchartTreeNode[]): OrgchartTreeNode[] {
	return [...nodes].sort((first, second) => compareRecords(first.record, second.record));
}

function recordGroupIDs(record: UserRecord): string[] {
	return Array.from(new Set([record.primaryGroupID ?? '', ...(record.groupIDs ?? [])].map((groupID) => groupID.trim()).filter(Boolean)));
}

function recordPrimaryGroupID(record: UserRecord): string {
	return (record.primaryGroupID ?? record.group ?? '').trim();
}

function normalizedID(value: string | undefined): string {
	return (value ?? '').trim();
}

function searchableText(record: UserRecord): string {
	return normalizedSearch([record.name, record.email, record.handle, record.jobTitle].filter(Boolean).join(' '));
}

function normalizedSearch(value: string): string {
	return value.trim().toLowerCase();
}

function sortedRecords(records: UserRecord[]): UserRecord[] {
	return [...records].sort(compareRecords);
}

function sortedColumnRecords(records: UserRecord[], recordDepths: Map<string, number>): UserRecord[] {
	return [...records].sort((first, second) => {
		const depthDifference = (recordDepths.get(first.userID) ?? 1) - (recordDepths.get(second.userID) ?? 1);
		if (depthDifference !== 0) return depthDifference;
		return compareRecords(first, second);
	});
}

function compareRecords(first: UserRecord, second: UserRecord): number {
	const hireDateDifference = (first.hireDate || '9999-12-31').localeCompare(second.hireDate || '9999-12-31');
	if (hireDateDifference !== 0) return hireDateDifference;
	return personLabel(first).localeCompare(personLabel(second));
}

function recordDepthMap(records: UserRecord[], roots: UserRecord[]): Map<string, number> {
	const recordsByID = new Map(records.map((record) => [record.userID, record]));
	return new Map(records.map((record) => [record.userID, recordDepth(record, recordsByID, new Set(roots.map((root) => root.userID)), new Set())]));
}

function recordDepth(record: UserRecord, recordsByID: Map<string, UserRecord>, rootIDs: Set<string>, visitedIDs: Set<string>): number {
	if (rootIDs.has(record.userID)) return 0;
	const supervisorID = normalizedID(record.supervisorID);
	if (!supervisorID) return 0;
	const supervisor = recordsByID.get(supervisorID);
	if (!supervisor || visitedIDs.has(record.userID)) return 0;
	const nextVisitedIDs = new Set(visitedIDs).add(record.userID);
	return recordDepth(supervisor, recordsByID, rootIDs, nextVisitedIDs) + 1;
}

function hasSupervisorCycle(record: UserRecord, records: UserRecord[]): boolean {
	const recordsByID = new Map(records.map((candidate) => [candidate.userID, candidate]));
	const visitedIDs = new Set<string>();
	let current = record;
	while (current.supervisorID) {
		const supervisorID = normalizedID(current.supervisorID);
		if (!supervisorID) return false;
		if (visitedIDs.has(supervisorID)) return true;
		visitedIDs.add(current.userID);
		const supervisor = recordsByID.get(supervisorID);
		if (!supervisor) return false;
		current = supervisor;
	}
	return false;
}

function personLabel(record: UserRecord): string {
	return record.name || record.email;
}
