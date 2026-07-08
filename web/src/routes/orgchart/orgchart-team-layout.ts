import type { UserRecord } from '../admin/admin-types';
import type { OrgchartTeamColumn } from './orgchart-directory-model';
import {
	orgchartColumnPaddingBottom,
	orgchartLevelGap,
	orgchartPersonNodeHeight,
	orgchartPersonNodeWidth,
	orgchartSiblingGap,
	orgchartTeamHeaderHeight
} from './orgchart-layout-constants';
import type { OrgchartPositionedEdge, OrgchartPositionedNode, OrgchartTeamLayout } from './orgchart-layout-types';
import { measureTree, measuredForestWidth, placeTree, type MeasuredTree } from './orgchart-tree-layout';

type MeasuredRootPlacement = {
	tree: MeasuredTree;
	left: number;
	centerX: number;
	supervisorID: string;
};

export function orgchartOrderedTeamLayouts(roots: UserRecord[], layouts: OrgchartTeamLayout[]): OrgchartTeamLayout[] {
	const rootOrderByUserID = new Map(roots.map((root, index) => [root.userID, index]));
	return layouts
		.map((layout, index) => ({ layout, index, rootOrder: layoutRootOrder(layout, rootOrderByUserID) }))
		.sort((first, second) => first.rootOrder - second.rootOrder || first.index - second.index)
		.map((entry) => entry.layout);
}

export function orgchartTeamLayout(column: OrgchartTeamColumn, roots: UserRecord[] = []): OrgchartTeamLayout {
	const measuredRoots = orderedMeasuredRoots(column.treeRoots.map(measureTree), roots);
	const forestWidth = measuredForestWidth(measuredRoots);
	const width = Math.max(orgchartPersonNodeWidth, forestWidth);
	const headerWidth = column.treeRoots.length > 1 ? width : orgchartPersonNodeWidth;
	const nodes: OrgchartPositionedNode[] = [];
	const edges: OrgchartPositionedEdge[] = [];
	const rootTop = column.treeRoots.length === 1 ? orgchartTeamHeaderHeight : orgchartTeamHeaderHeight + orgchartLevelGap;
	const rootPlacements = measuredRootPlacements(measuredRoots, (width - forestWidth) / 2);
	if (column.treeRoots.length > 1) {
		edges.push(...teamHeaderEdges(rootPlacements, width / 2, rootTop));
	}
	for (const placement of rootPlacements) {
		placeTree(placement.tree, placement.left, rootTop, column.treeRoots.length === 1, nodes, edges);
	}
	const bottom = Math.max(orgchartTeamHeaderHeight, ...nodes.map((node) => node.y + orgchartPersonNodeHeight));
	return {
		column,
		width,
		height: bottom + orgchartColumnPaddingBottom,
		headerX: (width - headerWidth) / 2,
		headerWidth,
		nodes,
		edges
	};
}

function layoutRootOrder(layout: OrgchartTeamLayout, rootOrderByUserID: Map<string, number>): number {
	const rootOrders = layout.column.treeRoots.flatMap((root) => {
		const supervisorID = root.record.supervisorID?.trim() ?? '';
		const rootOrder = rootOrderByUserID.get(supervisorID);
		return rootOrder === undefined ? [] : [rootOrder];
	});
	if (rootOrders.length === 0) return Number.MAX_SAFE_INTEGER;
	return Math.min(...rootOrders);
}

function orderedMeasuredRoots(roots: MeasuredTree[], rootRecords: UserRecord[]): MeasuredTree[] {
	const rootOrderByUserID = new Map(rootRecords.map((root, index) => [root.userID, index]));
	const knownSupervisorIDs = new Set(roots.map((root) => root.node.record.supervisorID?.trim() ?? '').filter((supervisorID) => rootOrderByUserID.has(supervisorID)));
	if (knownSupervisorIDs.size <= 1) return roots;
	return roots
		.map((root, index) => ({ root, index, supervisorOrder: rootOrderByUserID.get(root.node.record.supervisorID?.trim() ?? '') ?? Number.MAX_SAFE_INTEGER }))
		.sort((first, second) => first.supervisorOrder - second.supervisorOrder || first.index - second.index)
		.map((entry) => entry.root);
}

function measuredRootPlacements(roots: MeasuredTree[], firstLeft: number): MeasuredRootPlacement[] {
	let currentLeft = firstLeft;
	return roots.map((root) => {
		const placement = {
			tree: root,
			left: currentLeft,
			centerX: currentLeft + root.width / 2,
			supervisorID: root.node.record.supervisorID?.trim() ?? ''
		};
		currentLeft += root.width + orgchartSiblingGap;
		return placement;
	});
}

function teamHeaderEdges(placements: MeasuredRootPlacement[], headerCenterX: number, rootTop: number): OrgchartPositionedEdge[] {
	const groups = sharedSupervisorGroups(placements);
	if (groups.length <= 1) {
		return placements.map((placement) => teamHeaderEdge(headerCenterX, placement.centerX, rootTop));
	}
	return groups.flatMap((group) => groupedTeamHeaderEdges(group, rootTop));
}

function sharedSupervisorGroups(placements: MeasuredRootPlacement[]): MeasuredRootPlacement[][] {
	const supervisorIDs = new Set(placements.map((placement) => placement.supervisorID).filter(Boolean));
	if (supervisorIDs.size <= 1) return [];
	const groupsBySupervisorID = new Map<string, MeasuredRootPlacement[]>();
	for (const placement of placements) {
		groupsBySupervisorID.set(placement.supervisorID, [...(groupsBySupervisorID.get(placement.supervisorID) ?? []), placement]);
	}
	return [...groupsBySupervisorID.values()];
}

function groupedTeamHeaderEdges(placements: MeasuredRootPlacement[], rootTop: number): OrgchartPositionedEdge[] {
	const groupCenterX = rootPlacementGroupCenterX(placements);
	return placements.map((placement) => teamHeaderEdge(groupCenterX, placement.centerX, rootTop));
}

function rootPlacementGroupCenterX(placements: MeasuredRootPlacement[]): number {
	const left = Math.min(...placements.map((placement) => placement.centerX - orgchartPersonNodeWidth / 2));
	const right = Math.max(...placements.map((placement) => placement.centerX + orgchartPersonNodeWidth / 2));
	return (left + right) / 2;
}

function teamHeaderEdge(fromX: number, toX: number, rootTop: number): OrgchartPositionedEdge {
	return {
		fromX,
		fromY: orgchartTeamHeaderHeight,
		toX,
		toY: rootTop
	};
}
