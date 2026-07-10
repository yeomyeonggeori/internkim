import { describe, expect, test } from 'bun:test';
import {
	orgchartBoardMinimumWidth,
	orgchartColumnLeftPositions,
	orgchartDistributedColumnSpace,
	orgchartRootConnectorEdges,
	orgchartTeamLayout
} from '../../../src/routes/orgchart/orgchart-layout';
import { teamColumn, treeNode, userRecord } from './orgchart-layout-test-helpers';

describe('orgchart connector layout', () => {
	test('connects each top root to the team column it supervises', () => {
		const gamyeong = userRecord('gamyeong', 'Gamyeong');
		const pyobon = userRecord('pyobon', 'Pyobon');
		const pptx = userRecord('pptx', 'PPTX');
		const taskforce = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('yemun', 'Yemun', { supervisorID: pyobon.userID })),
			treeNode(userRecord('gyeonyang', 'Gyeonyang', { supervisorID: pyobon.userID }))
		]));
		const engineering = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('mohyeong', 'Mohyeong', { supervisorID: gamyeong.userID }))
		]));
		const operations = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('gaching', 'Gaching', { supervisorID: pptx.userID }))
		]));
		const layouts = [taskforce, engineering, operations];
		const columnWidths = layouts.map((layout) => layout.width);
		const columnSpace = orgchartDistributedColumnSpace(columnWidths, 1200);
		const columnLefts = orgchartColumnLeftPositions(columnWidths, columnSpace, 1200);

		expect(orgchartRootConnectorEdges([gamyeong, pyobon, pptx], layouts, columnLefts, 1200)).toEqual([
			{ fromX: 726, fromY: 0, toX: 726, toY: 96, middleY: 24 },
			{ fromX: 236, fromY: 0, toX: 236, toY: 96, middleY: 48 },
			{ fromX: 1090, fromY: 0, toX: 1090, toY: 96, middleY: 72 }
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
		const gamyeong = userRecord('gamyeong', 'Gamyeong');
		const pyobon = userRecord('pyobon', 'Pyobon');
		const pptx = userRecord('pptx', 'PPTX');
		const taskforce = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('yemun', 'Yemun', { supervisorID: pyobon.userID })),
			treeNode(userRecord('gyeonyang', 'Gyeonyang', { supervisorID: pyobon.userID }))
		]));
		const engineering = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('mohyeong', 'Mohyeong', { supervisorID: gamyeong.userID }))
		]));
		const operations = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('gaching', 'Gaching', { supervisorID: pptx.userID }))
		]));
		const layouts = [taskforce, engineering, operations];
		const columnWidths = layouts.map((layout) => layout.width);
		const boardWidth = orgchartBoardMinimumWidth(columnWidths);
		const columnSpace = orgchartDistributedColumnSpace(columnWidths, boardWidth);
		const columnLefts = orgchartColumnLeftPositions(columnWidths, columnSpace, boardWidth);

		expect(orgchartRootConnectorEdges([gamyeong, pyobon, pptx], layouts, columnLefts, boardWidth)).toEqual([
			{ fromX: 638, fromY: 0, toX: 638, toY: 96, middleY: 24 },
			{ fromX: 236, fromY: 0, toX: 236, toY: 96, middleY: 48 },
			{ fromX: 914, fromY: 0, toX: 914, toY: 96, middleY: 72 }
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
});
