import type { UserRecord } from '../admin/admin-types';
import type { OrgchartPositionedEdge, OrgchartRootTarget, OrgchartTeamLayout } from './orgchart-layout-types';
import { orgchartRootLayouts } from './orgchart-root-center-layout';
import { rootTargetsBySupervisorID } from './orgchart-root-targets';

export function orgchartRootConnectorEdges(
	roots: UserRecord[],
	layouts: OrgchartTeamLayout[],
	columnLefts: number[],
	width: number,
	rootCentersByUserID = new Map(orgchartRootLayouts(roots, layouts, columnLefts, width).map((root) => [root.record.userID, root.centerX]))
): OrgchartPositionedEdge[] {
	const rootOrderByUserID = new Map(roots.map((root, index) => [root.userID, index]));
	const rootTargetEntries = [...rootTargetsBySupervisorID(layouts, columnLefts).entries()]
		.sort((first, second) => (rootOrderByUserID.get(first[0]) ?? Number.MAX_SAFE_INTEGER) - (rootOrderByUserID.get(second[0]) ?? Number.MAX_SAFE_INTEGER));
	return rootTargetEntries.flatMap(([supervisorID, targets], index) => {
		const fromX = rootCentersByUserID.get(supervisorID);
		if (fromX === undefined) return [];
		const branchY = rootConnectorMiddleY(index, rootTargetEntries.length);
		return rootConnectorEdges(fromX, branchY, targets);
	});
}

function rootConnectorEdges(fromX: number, branchY: number, targets: OrgchartRootTarget[]): OrgchartPositionedEdge[] {
	const sortedTargets = [...targets].sort((first, second) => first.x - second.x);
	if (sortedTargets.length === 1) return [rootConnectorEdge(fromX, sortedTargets[0].x, branchY)];
	const branchLeft = Math.min(fromX, ...sortedTargets.map((target) => target.x));
	const branchRight = Math.max(fromX, ...sortedTargets.map((target) => target.x));
	return [
		{ fromX, fromY: 0, toX: fromX, toY: branchY },
		{ fromX: branchLeft, fromY: branchY, toX: branchRight, toY: branchY },
		...sortedTargets.map((target) => ({ fromX: target.x, fromY: branchY, toX: target.x, toY: 96 }))
	];
}

function rootConnectorEdge(fromX: number, toX: number, middleY: number): OrgchartPositionedEdge {
	return {
		fromX,
		fromY: 0,
		toX,
		toY: 96,
		middleY
	};
}

function rootConnectorMiddleY(index: number, total: number): number {
	if (total <= 1) return 48;
	return 24 + (48 * index) / (total - 1);
}
