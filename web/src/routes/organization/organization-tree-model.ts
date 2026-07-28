import { orderOrganizationPeopleByHierarchy } from '$lib/organization/person-order';
import type { OrgGroup, UserRecord } from '$lib/organization/types';

export type OrganizationOrganizationTreeNode = {
	id: string;
	name: string;
	parentID: string;
	depth: number;
	directRecords: UserRecord[];
	aggregateRecords: UserRecord[];
	memberCount: number;
};

export type OrganizationOrganizationTree = {
	root: OrganizationOrganizationTreeNode;
	nodes: OrganizationOrganizationTreeNode[];
};

export type OrganizationOrganizationTreeIndex = {
	nodeByID: Map<string, OrganizationOrganizationTreeNode>;
	groupIDsWithChildren: Set<string>;
};

export type OrganizationOrganizationMovePreview = {
	insertionIndex: number;
	depth: number;
	parentID: string;
};

export function organizationOrganizationTree(groups: OrgGroup[], records: UserRecord[], rootName: string): OrganizationOrganizationTree {
	const knownGroupIDs = new Set(groups.map((group) => group.id));
	const directRecordsByGroupID = recordsByPrimaryGroupID(records, knownGroupIDs);
	const childrenByParentID = groupsByParentID(groups, knownGroupIDs);
	const nodes: OrganizationOrganizationTreeNode[] = [];
	const visitedGroupIDs = new Set<string>();

	const buildBranch = (group: OrgGroup, depth: number): { node: OrganizationOrganizationTreeNode; flatNodes: OrganizationOrganizationTreeNode[] } => {
		visitedGroupIDs.add(group.id);
		const children = (childrenByParentID.get(group.id) ?? [])
			.filter((child) => !visitedGroupIDs.has(child.id))
			.map((child) => buildBranch(child, depth + 1));
		const directRecords = orderedRecords(directRecordsByGroupID.get(group.id) ?? []);
		const aggregateRecords = orderedRecords([...directRecords, ...children.flatMap((child) => child.node.aggregateRecords)]);
		const node = {
			id: group.id,
			name: group.name,
			parentID: normalizedParentID(group, knownGroupIDs),
			depth,
			directRecords,
			aggregateRecords,
			memberCount: aggregateRecords.length
		};
		return { node, flatNodes: [node, ...children.flatMap((child) => child.flatNodes)] };
	};

	for (const group of childrenByParentID.get('') ?? []) {
		if (!visitedGroupIDs.has(group.id)) nodes.push(...buildBranch(group, 0).flatNodes);
	}
	for (const group of groups) {
		if (!visitedGroupIDs.has(group.id)) nodes.push(...buildBranch({ ...group, parentID: '' }, 0).flatNodes);
	}

	const rootDirectRecords = orderedRecords(directRecordsByGroupID.get('') ?? []);
	return {
		root: {
			id: '',
			name: rootName,
			parentID: '',
			depth: -1,
			directRecords: rootDirectRecords,
			aggregateRecords: orderedRecords(records),
			memberCount: records.length
		},
		nodes
	};
}

export function organizationOrganizationTreeIndex(nodes: OrganizationOrganizationTreeNode[]): OrganizationOrganizationTreeIndex {
	const nodeByID = new Map(nodes.map((node) => [node.id, node]));
	const groupIDsWithChildren = new Set(nodes.flatMap((node) => (node.parentID ? [node.parentID] : [])));
	return { nodeByID, groupIDsWithChildren };
}

export function organizationOrganizationSubtreeIDs(nodes: OrganizationOrganizationTreeNode[], groupID: string): Set<string> {
	const groupIndex = nodes.findIndex((node) => node.id === groupID);
	if (groupIndex < 0) return new Set();
	const groupDepth = nodes[groupIndex].depth;
	const subtreeEndIndex = nodes.findIndex((node, index) => index > groupIndex && node.depth <= groupDepth);
	return new Set(nodes.slice(groupIndex, subtreeEndIndex < 0 ? nodes.length : subtreeEndIndex).map((node) => node.id));
}

export function moveOrganizationOrganization(groups: OrgGroup[], draggedGroupID: string, insertionIndex: number, requestedDepth: number): OrgGroup[] {
	const tree = organizationOrganizationTree(groups, [], '');
	const draggedIndex = tree.nodes.findIndex((node) => node.id === draggedGroupID);
	if (draggedIndex < 0) return groups;
	const draggedDepth = tree.nodes[draggedIndex].depth;
	const subtreeEndIndex = tree.nodes.findIndex((node, index) => index > draggedIndex && node.depth <= draggedDepth);
	const resolvedSubtreeEndIndex = subtreeEndIndex < 0 ? tree.nodes.length : subtreeEndIndex;
	const subtreeNodes = tree.nodes.slice(draggedIndex, resolvedSubtreeEndIndex);
	const subtreeGroupIDs = new Set(subtreeNodes.map((node) => node.id));
	const remainingNodes = tree.nodes.filter((node) => !subtreeGroupIDs.has(node.id));
	const preview = movePreviewForNodes(remainingNodes, insertionIndex, requestedDepth);
	const groupByID = new Map(groups.map((group) => [group.id, group]));
	const movedGroups = subtreeNodes.flatMap((node, index) => {
		const group = groupByID.get(node.id);
		if (!group) return [];
		return [{ ...group, ...(index === 0 ? { parentID: preview.parentID } : {}) }];
	});
	const remainingGroups = remainingNodes.flatMap((node) => {
		const group = groupByID.get(node.id);
		return group ? [group] : [];
	});
	return [...remainingGroups.slice(0, preview.insertionIndex), ...movedGroups, ...remainingGroups.slice(preview.insertionIndex)];
}

export function organizationOrganizationMovePreview(
	groups: OrgGroup[],
	draggedGroupID: string,
	insertionIndex: number,
	requestedDepth: number
): OrganizationOrganizationMovePreview {
	const tree = organizationOrganizationTree(groups, [], '');
	const draggedIndex = tree.nodes.findIndex((node) => node.id === draggedGroupID);
	if (draggedIndex < 0) return { insertionIndex: 0, depth: 0, parentID: '' };
	const draggedDepth = tree.nodes[draggedIndex].depth;
	const subtreeEndIndex = tree.nodes.findIndex((node, index) => index > draggedIndex && node.depth <= draggedDepth);
	const resolvedSubtreeEndIndex = subtreeEndIndex < 0 ? tree.nodes.length : subtreeEndIndex;
	const subtreeGroupIDs = new Set(tree.nodes.slice(draggedIndex, resolvedSubtreeEndIndex).map((node) => node.id));
	return movePreviewForNodes(tree.nodes.filter((node) => !subtreeGroupIDs.has(node.id)), insertionIndex, requestedDepth);
}

function groupsByParentID(groups: OrgGroup[], knownGroupIDs: Set<string>): Map<string, OrgGroup[]> {
	const result = new Map<string, OrgGroup[]>();
	for (const group of groups) {
		const parentID = normalizedParentID(group, knownGroupIDs);
		result.set(parentID, [...(result.get(parentID) ?? []), group]);
	}
	return result;
}

function normalizedParentID(group: OrgGroup, knownGroupIDs: Set<string>): string {
	const parentID = group.parentID?.trim() ?? '';
	return parentID && parentID !== group.id && knownGroupIDs.has(parentID) ? parentID : '';
}

function recordsByPrimaryGroupID(records: UserRecord[], knownGroupIDs: Set<string>): Map<string, UserRecord[]> {
	const result = new Map<string, UserRecord[]>();
	for (const record of records) {
		const groupID = record.groupID?.trim() ?? '';
		const resolvedGroupID = knownGroupIDs.has(groupID) ? groupID : '';
		result.set(resolvedGroupID, [...(result.get(resolvedGroupID) ?? []), record]);
	}
	return result;
}

function parentIDBeforeInsertion(nodes: OrganizationOrganizationTreeNode[], insertionIndex: number, depth: number): string {
	for (let index = insertionIndex - 1; index >= 0; index -= 1) {
		if (nodes[index].depth === depth - 1) return nodes[index].id;
	}
	return '';
}

function insertionIndexAfterDescendants(nodes: OrganizationOrganizationTreeNode[], insertionIndex: number, depth: number): number {
	let resolvedInsertionIndex = insertionIndex;
	while (resolvedInsertionIndex < nodes.length && nodes[resolvedInsertionIndex].depth > depth) resolvedInsertionIndex += 1;
	return resolvedInsertionIndex;
}

function movePreviewForNodes(nodes: OrganizationOrganizationTreeNode[], insertionIndex: number, requestedDepth: number): OrganizationOrganizationMovePreview {
	const boundedInsertionIndex = Math.max(0, Math.min(insertionIndex, nodes.length));
	const previousNode = nodes[boundedInsertionIndex - 1];
	const maximumDepth = previousNode ? previousNode.depth + 1 : 0;
	const depth = Math.max(0, Math.min(Math.trunc(requestedDepth), maximumDepth));
	const resolvedInsertionIndex = insertionIndexAfterDescendants(nodes, boundedInsertionIndex, depth);
	return {
		insertionIndex: resolvedInsertionIndex,
		depth,
		parentID: depth === 0 ? '' : parentIDBeforeInsertion(nodes, resolvedInsertionIndex, depth)
	};
}

function orderedRecords(records: UserRecord[]): UserRecord[] {
	return orderOrganizationPeopleByHierarchy(records);
}
