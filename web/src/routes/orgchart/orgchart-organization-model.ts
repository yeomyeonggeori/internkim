import type { OrgGroup, UserRecord } from '$lib/orgchart/types';
import { orgchartOrganizationTree, type OrgchartOrganizationTreeNode } from './orgchart-organization-tree-model';

export type OrgchartOrganizationSection = {
	id: string;
	name: string;
	depth: number;
	records: UserRecord[];
	memberCount: number;
	companyResponsibleUserID?: string;
	responsibleUserID?: string;
};

export function orgchartOrganizationSections(
	records: UserRecord[],
	groups: OrgGroup[],
	rootName: string,
	selectedOrganizationID = '',
	countRecords: UserRecord[] = records
): OrgchartOrganizationSection[] {
	const tree = orgchartOrganizationTree(groups, records, rootName);
	const countTree = countRecords === records ? tree : orgchartOrganizationTree(groups, countRecords, rootName);
	const memberCountByOrganizationID = new Map([
		['', countTree.root.memberCount],
		...countTree.nodes.map((node) => [node.id, node.memberCount] as const)
	]);
	const companyResponsibleUserID = orgchartResponsibleUserID(countTree.root.aggregateRecords);
	const responsibleUserIDByOrganizationID = new Map([
		['', undefined],
		...countTree.nodes.map((node) => [node.id, orgchartResponsibleUserID(node.directRecords)] as const)
	]);
	if (!selectedOrganizationID) {
		return [
			sectionFromNode(tree.root, 0, memberCountByOrganizationID, responsibleUserIDByOrganizationID, companyResponsibleUserID),
			...tree.nodes.map((node) => sectionFromNode(node, node.depth + 1, memberCountByOrganizationID, responsibleUserIDByOrganizationID, companyResponsibleUserID))
		];
	}
	const selectedIndex = tree.nodes.findIndex((node) => node.id === selectedOrganizationID);
	if (selectedIndex < 0) return [];
	const selectedDepth = tree.nodes[selectedIndex].depth;
	const subtreeEndIndex = tree.nodes.findIndex((node, index) => index > selectedIndex && node.depth <= selectedDepth);
	const resolvedSubtreeEndIndex = subtreeEndIndex < 0 ? tree.nodes.length : subtreeEndIndex;
	return tree.nodes
		.slice(selectedIndex, resolvedSubtreeEndIndex)
		.map((node) => sectionFromNode(node, node.depth - selectedDepth, memberCountByOrganizationID, responsibleUserIDByOrganizationID, companyResponsibleUserID));
}

function sectionFromNode(
	node: OrgchartOrganizationTreeNode,
	depth: number,
	memberCountByOrganizationID: Map<string, number>,
	responsibleUserIDByOrganizationID: Map<string, string | undefined>,
	companyResponsibleUserID: string | undefined
): OrgchartOrganizationSection {
	return {
		id: node.id,
		name: node.name,
		depth,
		records: node.directRecords,
		memberCount: memberCountByOrganizationID.get(node.id) ?? 0,
		companyResponsibleUserID,
		responsibleUserID: responsibleUserIDByOrganizationID.get(node.id)
	};
}

function orgchartResponsibleUserID(records: UserRecord[]): string | undefined {
	const userIDs = new Set(records.map((record) => record.userID));
	const candidates = records.filter((record) => {
		const supervisorID = record.supervisorID?.trim() ?? '';
		return !supervisorID || !userIDs.has(supervisorID);
	});
	return candidates.length === 1 ? candidates[0].userID : undefined;
}
