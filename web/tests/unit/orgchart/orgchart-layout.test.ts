import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/routes/admin/admin-types';
import type { OrgchartTeamColumn, OrgchartTreeNode } from '../../../src/routes/orgchart/orgchart-directory-model';
import {
	orgchartBoardMinimumWidth,
	orgchartColumnCenterPositions,
	orgchartColumnLeftPositions,
	orgchartDistributedColumnSpace,
	orgchartEdgePath,
	orgchartFitZoom,
	orgchartLevelGap,
	orgchartOrderedTeamLayouts,
	orgchartPersonNodeHeight,
	orgchartPersonNodeWidth,
	orgchartRootCenterPositions,
	orgchartRootConnectorEdges,
	orgchartRootLayouts,
	orgchartRootNodeWidth,
	orgchartRootRowMinimumWidth,
	orgchartSiblingGap,
	orgchartTeamHeaderHeight,
	orgchartTeamLayout
} from '../../../src/routes/orgchart/orgchart-layout';

function userRecord(userID: string, name: string, fields: Partial<UserRecord> = {}): UserRecord {
	return {
		userID,
		handle: userID,
		name,
		email: `${userID}@example.com`,
		role: 'member',
		...fields
	};
}

function treeNode(record: UserRecord, children: OrgchartTreeNode[] = []): OrgchartTreeNode {
	return {
		record,
		depth: 1,
		children
	};
}

function teamColumn(treeRoots: OrgchartTreeNode[], fields: Partial<OrgchartTeamColumn> = {}): OrgchartTeamColumn {
	return {
		id: fields.id ?? 'product',
		name: fields.name ?? '제품팀',
		records: treeRoots.map((root) => root.record),
		nodes: treeRoots.map((root) => ({ record: root.record, depth: root.depth })),
		treeRoots,
		isUnassigned: fields.isUnassigned ?? false
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
			['ada', 126, rootTop, true],
			['grace', 0, childTop, false],
			['tess', 0, grandChildTop, false],
			['lin', orgchartPersonNodeWidth + orgchartSiblingGap, childTop, false]
		]);
		expect(layout.edges[0]).toEqual({
			fromX: 236,
			fromY: rootTop + orgchartPersonNodeHeight,
			toX: 110,
			toY: childTop
		});
		expect(orgchartEdgePath(layout.edges[0])).toBe('M 236 152 V 168 H 110 V 184');
		expect(layout.headerWidth).toBe(orgchartPersonNodeWidth);
	});

	test('stretches the organization header across multiple leaders', () => {
		const firstLeader = treeNode(userRecord('first-leader', 'First'));
		const secondLeader = treeNode(userRecord('second-leader', 'Second'));
		const thirdLeader = treeNode(userRecord('third-leader', 'Third'));

		const layout = orgchartTeamLayout(teamColumn([firstLeader, secondLeader, thirdLeader]));
		const rootTop = orgchartTeamHeaderHeight + orgchartLevelGap;

		expect(layout.width).toBe(orgchartPersonNodeWidth * 3 + orgchartSiblingGap * 2);
		expect(layout.headerX).toBe(0);
		expect(layout.headerWidth).toBe(layout.width);
		expect(layout.nodes.map((node) => [node.record.userID, node.x, node.y, node.isAttachedToHeader])).toEqual([
			['first-leader', 0, rootTop, false],
			['second-leader', orgchartPersonNodeWidth + orgchartSiblingGap, rootTop, false],
			['third-leader', (orgchartPersonNodeWidth + orgchartSiblingGap) * 2, rootTop, false]
		]);
	});

	test('distributes connector positions across the available board width', () => {
		const columnWidths = [220, 456, 220];
		const boardWidth = 1200;
		const boardMinimumWidth = orgchartBoardMinimumWidth(columnWidths);
		const columnSpace = orgchartDistributedColumnSpace(columnWidths, boardWidth);
		const columnLefts = orgchartColumnLeftPositions(columnWidths, columnSpace, boardWidth);
		const connectorPositions = orgchartColumnCenterPositions(columnWidths, columnSpace, boardWidth);

		expect(boardMinimumWidth).toBe(1008);
		expect(columnSpace).toBe(152);
		expect(columnLefts).toEqual([0, 372, 980]);
		expect(connectorPositions).toEqual([110, 600, 1090]);
	});

	test('centers a single column connector in the board', () => {
		expect(orgchartColumnLeftPositions([220], 0, 1200)).toEqual([490]);
		expect(orgchartColumnCenterPositions([220], 0, 1200)).toEqual([600]);
	});

	test('positions root centers with the rendered root node width', () => {
		expect(orgchartRootNodeWidth).toBe(orgchartPersonNodeWidth);
		expect(orgchartRootRowMinimumWidth(3)).toBe(orgchartRootNodeWidth * 3 + orgchartSiblingGap * 2);
		expect(orgchartRootCenterPositions(3, 1200)).toEqual([
			(1200 - orgchartRootNodeWidth * 3 - orgchartSiblingGap * 2) / 2 + orgchartRootNodeWidth / 2,
			600,
			(1200 + orgchartRootNodeWidth * 3 + orgchartSiblingGap * 2) / 2 - orgchartRootNodeWidth / 2
		]);
	});

	test('keeps root cards separated when their target columns overlap', () => {
		const firstRoot = userRecord('first-root', 'First Root');
		const secondRoot = userRecord('second-root', 'Second Root');
		const sharedTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('first-leader', 'First Leader', { supervisorID: firstRoot.userID })),
			treeNode(userRecord('second-leader', 'Second Leader', { supervisorID: secondRoot.userID }))
		]));
		const width = orgchartRootRowMinimumWidth(2);
		const columnLefts = orgchartColumnLeftPositions([sharedTeam.width], 0, width);

		expect(orgchartRootLayouts([firstRoot, secondRoot], [sharedTeam], columnLefts, width).map((root) => root.centerX)).toEqual([
			orgchartRootNodeWidth / 2,
			orgchartRootNodeWidth / 2 + orgchartRootNodeWidth + orgchartSiblingGap
		]);
	});

	test('connects each top root to the team column it supervises', () => {
		const dongha = userRecord('dongha', 'Dongha');
		const yeomyeong = userRecord('yeomyeong', 'Yeomyeong');
		const pptx = userRecord('pptx', 'PPTX');
		const taskforce = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('sungjae', 'Sungjae', { supervisorID: yeomyeong.userID })),
			treeNode(userRecord('seeun', 'Seeun', { supervisorID: yeomyeong.userID }))
		]));
		const engineering = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('chanhee', 'Chanhee', { supervisorID: dongha.userID }))
		]));
		const operations = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('seokmin', 'Seokmin', { supervisorID: pptx.userID }))
		]));
		const layouts = [taskforce, engineering, operations];
		const columnWidths = layouts.map((layout) => layout.width);
		const columnSpace = orgchartDistributedColumnSpace(columnWidths, 1200);
		const columnLefts = orgchartColumnLeftPositions(columnWidths, columnSpace, 1200);

		expect(orgchartRootConnectorEdges([dongha, yeomyeong, pptx], layouts, columnLefts, 1200)).toEqual([
			{ fromX: 236, fromY: 0, toX: 236, toY: 96, middleY: 24 },
			{ fromX: 726, fromY: 0, toX: 726, toY: 96, middleY: 48 },
			{ fromX: 1090, fromY: 0, toX: 1090, toY: 96, middleY: 72 }
		]);
	});

	test('groups team columns by the top root that supervises them', () => {
		const firstRoot = userRecord('first-root', 'First Root');
		const secondRoot = userRecord('second-root', 'Second Root');
		const firstTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('first-leader', 'First Leader', { supervisorID: firstRoot.userID }))
		], { id: 'first-team' }));
		const secondTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('second-leader', 'Second Leader', { supervisorID: secondRoot.userID }))
		], { id: 'second-team' }));
		const anotherFirstTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('another-first-leader', 'Another First Leader', { supervisorID: firstRoot.userID }))
		], { id: 'another-first-team' }));

		expect(orgchartOrderedTeamLayouts([firstRoot, secondRoot], [firstTeam, secondTeam, anotherFirstTeam]).map((layout) => layout.column.id)).toEqual([
			'first-team',
			'another-first-team',
			'second-team'
		]);
	});

	test('draws one trunk and short branches when a root supervises multiple teams', () => {
		const root = userRecord('root', 'Root');
		const firstTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('first-leader', 'First Leader', { supervisorID: root.userID }))
		]));
		const secondTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('second-leader', 'Second Leader', { supervisorID: root.userID }))
		]));
		const layouts = [firstTeam, secondTeam];
		const columnWidths = layouts.map((layout) => layout.width);
		const boardWidth = orgchartBoardMinimumWidth(columnWidths);
		const columnSpace = orgchartDistributedColumnSpace(columnWidths, boardWidth);
		const columnLefts = orgchartColumnLeftPositions(columnWidths, columnSpace, boardWidth);

		expect(orgchartRootConnectorEdges([root], layouts, columnLefts, boardWidth)).toEqual([
			{ fromX: 248, fromY: 0, toX: 248, toY: 48 },
			{ fromX: 110, fromY: 48, toX: 386, toY: 48 },
			{ fromX: 110, fromY: 48, toX: 110, toY: 96 },
			{ fromX: 386, fromY: 48, toX: 386, toY: 96 }
		]);
	});

	test('keeps compact root connectors vertical when root cards do not overlap', () => {
		const dongha = userRecord('dongha', 'Dongha');
		const yeomyeong = userRecord('yeomyeong', 'Yeomyeong');
		const pptx = userRecord('pptx', 'PPTX');
		const taskforce = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('sungjae', 'Sungjae', { supervisorID: yeomyeong.userID })),
			treeNode(userRecord('seeun', 'Seeun', { supervisorID: yeomyeong.userID }))
		]));
		const engineering = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('chanhee', 'Chanhee', { supervisorID: dongha.userID }))
		]));
		const operations = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('seokmin', 'Seokmin', { supervisorID: pptx.userID }))
		]));
		const layouts = [taskforce, engineering, operations];
		const columnWidths = layouts.map((layout) => layout.width);
		const boardWidth = orgchartBoardMinimumWidth(columnWidths);
		const columnSpace = orgchartDistributedColumnSpace(columnWidths, boardWidth);
		const columnLefts = orgchartColumnLeftPositions(columnWidths, columnSpace, boardWidth);

		expect(orgchartRootConnectorEdges([dongha, yeomyeong, pptx], layouts, columnLefts, boardWidth)).toEqual([
			{ fromX: 236, fromY: 0, toX: 236, toY: 96, middleY: 24 },
			{ fromX: 638, fromY: 0, toX: 638, toY: 96, middleY: 48 },
			{ fromX: 914, fromY: 0, toX: 914, toY: 96, middleY: 72 }
		]);
	});

	test('keeps target roots aligned when another root has no target team', () => {
		const yeomyeong = userRecord('yeomyeong', 'Yeomyeong');
		const dongha = userRecord('dongha', 'Dongha');
		const pptx = userRecord('pptx', 'PPTX');
		const taskforce = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('sungjae', 'Sungjae', { supervisorID: yeomyeong.userID }))
		]));
		const engineering = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('chanhee', 'Chanhee', { supervisorID: dongha.userID }))
		]));
		const layouts = [taskforce, engineering];
		const columnWidths = layouts.map((layout) => layout.width);
		const boardWidth = Math.max(orgchartBoardMinimumWidth(columnWidths), orgchartRootRowMinimumWidth(3));
		const columnSpace = orgchartDistributedColumnSpace(columnWidths, boardWidth);
		const columnLefts = orgchartColumnLeftPositions(columnWidths, columnSpace, boardWidth);

		expect(orgchartRootLayouts([yeomyeong, dongha, pptx], layouts, columnLefts, boardWidth).map((root) => [root.record.userID, root.centerX])).toEqual([
			['yeomyeong', 110],
			['dongha', 614],
			['pptx', 362]
		]);
		expect(orgchartRootConnectorEdges([yeomyeong, dongha, pptx], layouts, columnLefts, boardWidth)).toEqual([
			{ fromX: 110, fromY: 0, toX: 110, toY: 96, middleY: 24 },
			{ fromX: 614, fromY: 0, toX: 614, toY: 96, middleY: 72 }
		]);
	});

	test('connects individual leaders when one team has multiple root supervisors', () => {
		const firstRoot = userRecord('first-root', 'First Root');
		const secondRoot = userRecord('second-root', 'Second Root');
		const sharedTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('first-leader', 'First Leader', { supervisorID: firstRoot.userID })),
			treeNode(userRecord('second-leader', 'Second Leader', { supervisorID: secondRoot.userID }))
		]));
		const columnLefts = orgchartColumnLeftPositions([sharedTeam.width], 0, 1000);

		expect(orgchartRootConnectorEdges([firstRoot, secondRoot], [sharedTeam], columnLefts, 1000)).toEqual([
			{ fromX: 374, fromY: 0, toX: 374, toY: 96, middleY: 24 },
			{ fromX: 626, fromY: 0, toX: 626, toY: 96, middleY: 72 }
		]);
	});

	test('fits org chart zoom to the available viewport down to the minimum zoom', () => {
		expect(orgchartFitZoom({ boardWidth: 900, boardHeight: 500, viewportWidth: 1200, viewportHeight: 800 })).toBe(100);
		expect(orgchartFitZoom({ boardWidth: 1200, boardHeight: 500, viewportWidth: 960, viewportHeight: 800 })).toBe(80);
		expect(orgchartFitZoom({ boardWidth: 900, boardHeight: 900, viewportWidth: 1200, viewportHeight: 720 })).toBe(80);
		expect(orgchartFitZoom({ boardWidth: 2400, boardHeight: 1200, viewportWidth: 960, viewportHeight: 600 })).toBe(70);
	});
});
