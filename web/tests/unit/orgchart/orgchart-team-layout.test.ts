import { describe, expect, test } from 'bun:test';
import {
	orgchartEdgePath,
	orgchartLevelGap,
	orgchartOrderedTeamLayouts,
	orgchartPersonNodeHeight,
	orgchartPersonNodeWidth,
	orgchartSiblingGap,
	orgchartTeamHeaderHeight,
	orgchartTeamLayout
} from '../../../src/routes/orgchart/orgchart-layout';
import { teamColumn, treeNode, userRecord } from './orgchart-layout-test-helpers';

describe('orgchart team layout', () => {
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

	test('branches shared team members only within their direct root group', () => {
		const firstRoot = userRecord('first-root', 'First Root');
		const secondRoot = userRecord('second-root', 'Second Root');
		const sharedTeam = orgchartTeamLayout(teamColumn([
			treeNode(userRecord('second-leader', 'Second Leader', { supervisorID: secondRoot.userID })),
			treeNode(userRecord('first-leader', 'First Leader', { supervisorID: firstRoot.userID })),
			treeNode(userRecord('second-new-leader', 'Second New Leader', { supervisorID: secondRoot.userID }))
		]), [firstRoot, secondRoot]);

		expect(sharedTeam.width).toBe(orgchartPersonNodeWidth * 3 + orgchartSiblingGap * 2);
		expect(sharedTeam.headerX).toBe(0);
		expect(sharedTeam.headerWidth).toBe(sharedTeam.width);
		expect(sharedTeam.nodes.map((node) => [node.record.userID, node.x])).toEqual([
			['first-leader', 0],
			['second-leader', orgchartPersonNodeWidth + orgchartSiblingGap],
			['second-new-leader', (orgchartPersonNodeWidth + orgchartSiblingGap) * 2]
		]);
		expect(sharedTeam.edges).toEqual([
			{ fromX: 110, fromY: orgchartTeamHeaderHeight, toX: 110, toY: orgchartTeamHeaderHeight + orgchartLevelGap },
			{ fromX: 488, fromY: orgchartTeamHeaderHeight, toX: 362, toY: orgchartTeamHeaderHeight + orgchartLevelGap },
			{ fromX: 488, fromY: orgchartTeamHeaderHeight, toX: 614, toY: orgchartTeamHeaderHeight + orgchartLevelGap }
		]);
	});
});
