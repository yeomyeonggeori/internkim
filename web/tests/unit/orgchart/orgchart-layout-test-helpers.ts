import type { UserRecord } from '../../../src/routes/admin/admin-types';
import type { OrgchartTeamColumn, OrgchartTreeNode } from '../../../src/routes/orgchart/orgchart-directory-model';

export function userRecord(userID: string, name: string, fields: Partial<UserRecord> = {}): UserRecord {
	return {
		userID,
		handle: userID,
		name,
		email: `${userID}@example.com`,
		role: 'member',
		...fields
	};
}

export function treeNode(record: UserRecord, children: OrgchartTreeNode[] = []): OrgchartTreeNode {
	return {
		record,
		depth: 1,
		children
	};
}

export function teamColumn(treeRoots: OrgchartTreeNode[], fields: Partial<OrgchartTeamColumn> = {}): OrgchartTeamColumn {
	return {
		id: fields.id ?? 'product',
		name: fields.name ?? '제품팀',
		records: treeRoots.map((root) => root.record),
		nodes: treeRoots.map((root) => ({ record: root.record, depth: root.depth })),
		treeRoots,
		isUnassigned: fields.isUnassigned ?? false
	};
}
