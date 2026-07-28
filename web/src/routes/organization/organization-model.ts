import type { OrgGroup, UserRecord } from '$lib/organization/types';
import type { Locale } from '../../lib/i18n/locale.svelte';
import { compareOrganizationGroups } from './organization-group-order';
import { organizationOrganizationTree, type OrganizationOrganizationTreeNode } from './organization-tree-model';

export type OrganizationOrganizationSection = {
	id: string;
	name: string;
	depth: number;
	records: UserRecord[];
	memberCount: number;
	companyResponsibleUserID?: string;
	responsibleUserID?: string;
};

export function organizationOrganizationSections(
	records: UserRecord[],
	groups: OrgGroup[],
	rootName: string,
	selectedOrganizationID = '',
	countRecords: UserRecord[] = records,
	locale: Locale = 'ko'
): OrganizationOrganizationSection[] {
	const orderedGroups = groups.map((group) => ({ ...group, isUnassigned: false })).sort(compareOrganizationGroups(countRecords, locale));
	const tree = organizationOrganizationTree(orderedGroups, records, rootName);
	const countTree = countRecords === records ? tree : organizationOrganizationTree(orderedGroups, countRecords, rootName);
	const memberCountByOrganizationID = new Map([
		['', countTree.root.memberCount],
		...countTree.nodes.map((node) => [node.id, node.memberCount] as const)
	]);
	const companyResponsibleUserID = organizationCompanyResponsibleUserID(countTree.root.aggregateRecords);
	const responsibleUserIDByOrganizationID = new Map([
		['', undefined],
		...countTree.nodes.map((node) => [node.id, organizationResponsibleUserID(node.directRecords)] as const)
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
	node: OrganizationOrganizationTreeNode,
	depth: number,
	memberCountByOrganizationID: Map<string, number>,
	responsibleUserIDByOrganizationID: Map<string, string | undefined>,
	companyResponsibleUserID: string | undefined
): OrganizationOrganizationSection {
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

function organizationCompanyResponsibleUserID(records: UserRecord[]): string | undefined {
	const candidates = records.filter((record) => !(record.supervisorID?.trim() ?? ''));
	return candidates.length === 1 ? candidates[0].userID : undefined;
}

function organizationResponsibleUserID(records: UserRecord[]): string | undefined {
	const userIDs = new Set(records.map((record) => record.userID));
	const candidates = records.filter((record) => {
		const supervisorID = record.supervisorID?.trim() ?? '';
		return !supervisorID || !userIDs.has(supervisorID);
	});
	return candidates.length === 1 ? candidates[0].userID : undefined;
}
