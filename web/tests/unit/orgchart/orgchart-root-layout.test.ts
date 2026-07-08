import { describe, expect, test } from 'bun:test';
import {
	orgchartBoardMinimumWidth,
	orgchartColumnLeftPositions,
	orgchartDistributedColumnSpace,
	orgchartPersonNodeWidth,
	orgchartRootBlockLayout,
	orgchartRootCenterPositions,
	orgchartRootConnectorEdges,
	orgchartRootLayouts,
	orgchartRootNodeWidth,
	orgchartRootRowMinimumWidth,
	orgchartSiblingGap,
	orgchartTeamLayout
} from '../../../src/routes/orgchart/orgchart-layout';
import { teamColumn, treeNode, userRecord } from './orgchart-layout-test-helpers';

describe('orgchart root layout', () => {
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

	test('centers roots over their own child block and reserves an empty block for roots without reports', () => {
		const dongha = userRecord('dongha', 'Dongha');
		const yeomyeong = userRecord('yeomyeong', 'Yeomyeong');
		const pptx = userRecord('pptx', 'PPTX');
		const engineering = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('chanhee', 'Chanhee', { supervisorID: dongha.userID }))
		]));
		const taskforce = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('sungjae', 'Sungjae', { supervisorID: yeomyeong.userID }))
		]));
		const operations = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('seokmin', 'Seokmin', { supervisorID: yeomyeong.userID }))
		]));
		const layouts = [engineering, taskforce, operations];
		const blockLayout = orgchartRootBlockLayout([dongha, yeomyeong, pptx], layouts);
		const rootCenters = orgchartRootLayouts([dongha, yeomyeong, pptx], layouts, blockLayout.columnLefts, blockLayout.width, blockLayout.rootCentersByUserID).map((root) => root.centerX);

		expect(blockLayout.width).toBe(1048);
		expect(blockLayout.columnLefts).toEqual([0, 276, 552]);
		expect(rootCenters).toEqual([110, 524, 938]);
		for (let index = 1; index < rootCenters.length; index += 1) {
			expect(rootCenters[index] - rootCenters[index - 1] >= orgchartRootNodeWidth + orgchartSiblingGap).toBe(true);
		}
	});

	test('centers shared team roots over their own direct reports in block layout', () => {
		const firstRoot = userRecord('first-root', 'First Root');
		const secondRoot = userRecord('second-root', 'Second Root');
		const sharedTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('first-leader', 'First Leader', { supervisorID: firstRoot.userID })),
			treeNode(userRecord('second-leader', 'Second Leader', { supervisorID: secondRoot.userID }))
		]), [firstRoot, secondRoot]);
		const blockLayout = orgchartRootBlockLayout([firstRoot, secondRoot], [sharedTeam]);
		const rootCenters = orgchartRootLayouts([firstRoot, secondRoot], [sharedTeam], blockLayout.columnLefts, blockLayout.width, blockLayout.rootCentersByUserID).map((root) => root.centerX);

		expect(blockLayout.width).toBe(orgchartRootRowMinimumWidth(2));
		expect(blockLayout.columnLefts).toEqual([0]);
		expect(rootCenters).toEqual([orgchartRootNodeWidth / 2, orgchartRootNodeWidth / 2 + orgchartRootNodeWidth + orgchartSiblingGap]);
		expect(orgchartRootConnectorEdges([firstRoot, secondRoot], [sharedTeam], blockLayout.columnLefts, blockLayout.width, blockLayout.rootCentersByUserID)).toEqual([
			{ fromX: 110, fromY: 0, toX: 110, toY: 96, middleY: 24 },
			{ fromX: 362, fromY: 0, toX: 362, toY: 96, middleY: 72 }
		]);
	});

	test('centers shared team roots over grouped direct reports in block layout', () => {
		const firstRoot = userRecord('first-root', 'First Root');
		const secondRoot = userRecord('second-root', 'Second Root');
		const sharedTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('second-leader', 'Second Leader', { supervisorID: secondRoot.userID })),
			treeNode(userRecord('first-leader', 'First Leader', { supervisorID: firstRoot.userID })),
			treeNode(userRecord('second-new-leader', 'Second New Leader', { supervisorID: secondRoot.userID }))
		]), [firstRoot, secondRoot]);
		const blockLayout = orgchartRootBlockLayout([firstRoot, secondRoot], [sharedTeam]);
		const rootCenters = orgchartRootLayouts([firstRoot, secondRoot], [sharedTeam], blockLayout.columnLefts, blockLayout.width, blockLayout.rootCentersByUserID).map((root) => root.centerX);

		expect(blockLayout.width).toBe(sharedTeam.width);
		expect(blockLayout.columnLefts).toEqual([0]);
		expect(rootCenters).toEqual([110, 488]);
		expect(orgchartRootConnectorEdges([firstRoot, secondRoot], [sharedTeam], blockLayout.columnLefts, blockLayout.width, blockLayout.rootCentersByUserID)).toEqual([
			{ fromX: 110, fromY: 0, toX: 110, toY: 96, middleY: 24 },
			{ fromX: 488, fromY: 0, toX: 488, toY: 96, middleY: 72 }
		]);
	});
});
