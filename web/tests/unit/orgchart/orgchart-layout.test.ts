import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/routes/admin/admin-types';
import type { OrgchartTeamColumn, OrgchartTreeNode } from '../../../src/routes/orgchart/orgchart-directory-model';
import {
	orgchartBoardMinimumWidth,
	orgchartColumnCenterPositions,
	orgchartDistributedColumnSpace,
	orgchartEdgePath,
	orgchartLevelGap,
	orgchartPersonNodeHeight,
	orgchartPersonNodeWidth,
	orgchartSiblingGap,
	orgchartTeamHeaderHeight,
	orgchartTeamLayout
} from '../../../src/routes/orgchart/orgchart-layout';

function userRecord(userID: string, name: string): UserRecord {
	return {
		userID,
		handle: userID,
		name,
		email: `${userID}@example.com`,
		role: 'member'
	};
}

function treeNode(record: UserRecord, children: OrgchartTreeNode[] = []): OrgchartTreeNode {
	return {
		record,
		depth: 1,
		children
	};
}

function teamColumn(treeRoots: OrgchartTreeNode[]): OrgchartTeamColumn {
	return {
		id: 'product',
		name: '제품팀',
		records: treeRoots.map((root) => root.record),
		nodes: treeRoots.map((root) => ({ record: root.record, depth: root.depth })),
		treeRoots,
		isUnassigned: false
	};
}

describe('orgchart layout', () => {
	test('positions nested team members and connects card edges', () => {
		const grace = treeNode(userRecord('grace', 'Grace'), [treeNode(userRecord('tess', 'Tess'))]);
		const lin = treeNode(userRecord('lin', 'Lin'));
		const ada = treeNode(userRecord('ada', 'Ada'), [grace, lin]);

		const layout = orgchartTeamLayout(teamColumn([ada]));
		const rootTop = orgchartTeamHeaderHeight;
		const childTop = rootTop + orgchartPersonNodeHeight + orgchartLevelGap;
		const grandChildTop = childTop + orgchartPersonNodeHeight + orgchartLevelGap;

		expect(layout.width).toBe(orgchartPersonNodeWidth * 2 + orgchartSiblingGap);
		expect(layout.nodes.map((node) => [node.record.userID, node.x, node.y, node.isAttachedToHeader])).toEqual([
			['ada', 118, rootTop, true],
			['grace', 0, childTop, false],
			['tess', 0, grandChildTop, false],
			['lin', orgchartPersonNodeWidth + orgchartSiblingGap, childTop, false]
		]);
		expect(layout.edges[0]).toEqual({
			fromX: 228,
			fromY: rootTop + orgchartPersonNodeHeight,
			toX: 110,
			toY: childTop
		});
		expect(orgchartEdgePath(layout.edges[0])).toBe('M 228 152 V 168 H 110 V 184');
	});

	test('distributes connector positions across the available board width', () => {
		const columnWidths = [220, 456, 220];
		const boardWidth = 1200;
		const boardMinimumWidth = orgchartBoardMinimumWidth(columnWidths);
		const columnSpace = orgchartDistributedColumnSpace(columnWidths, boardWidth);
		const connectorPositions = orgchartColumnCenterPositions(columnWidths, columnSpace, boardWidth);

		expect(boardMinimumWidth).toBe(952);
		expect(columnSpace).toBe(152);
		expect(connectorPositions).toEqual([110, 600, 1090]);
	});

	test('centers a single column connector in the board', () => {
		expect(orgchartColumnCenterPositions([220], 0, 1200)).toEqual([600]);
	});
});
