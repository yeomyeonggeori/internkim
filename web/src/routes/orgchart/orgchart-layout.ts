import type { UserRecord } from '../admin/admin-types';
import type { OrgchartTeamColumn, OrgchartTreeNode } from './orgchart-directory-model';

type MeasuredTree = {
	node: OrgchartTreeNode;
	width: number;
	children: MeasuredTree[];
};

export type OrgchartPositionedNode = {
	record: UserRecord;
	x: number;
	y: number;
	isAttachedToHeader: boolean;
};

export type OrgchartPositionedEdge = {
	fromX: number;
	fromY: number;
	toX: number;
	toY: number;
	middleY?: number;
};

export type OrgchartTeamLayout = {
	column: OrgchartTeamColumn;
	width: number;
	height: number;
	headerX: number;
	headerWidth: number;
	nodes: OrgchartPositionedNode[];
	edges: OrgchartPositionedEdge[];
};

export type OrgchartPositionedRoot = {
	record: UserRecord;
	x: number;
	centerX: number;
};

export type OrgchartFitZoomInput = {
	boardWidth: number;
	boardHeight: number;
	viewportWidth: number;
	viewportHeight: number;
	minimumZoom?: number;
	maximumZoom?: number;
};

type OrgchartRootTarget = {
	supervisorID: string;
	x: number;
	left: number;
	right: number;
};

type OrgchartRootCenterCandidate = {
	index: number;
	center: number;
	fallbackCenter: number;
	hasTargets: boolean;
};

export const orgchartMinimumZoom = 70;
export const orgchartDefaultZoom = 100;
export const orgchartMaximumZoom = 130;
export const orgchartPersonNodeWidth = 220;
export const orgchartRootNodeWidth = orgchartPersonNodeWidth;
export const orgchartPersonNodeHeight = 96;
export const orgchartTeamHeaderHeight = 56;
export const orgchartColumnGap = 56;
export const orgchartSiblingGap = 32;
export const orgchartLevelGap = 32;
export const orgchartColumnPaddingBottom = 4;

export function orgchartRootRowMinimumWidth(rootCount: number): number {
	if (rootCount <= 0) return 0;
	return rootCount * orgchartRootNodeWidth + (rootCount - 1) * orgchartSiblingGap;
}

export function orgchartBoardMinimumWidth(columnWidths: number[]): number {
	if (columnWidths.length === 0) return 0;
	return orgchartColumnEdgePadding(columnWidths, 'start') + columnWidths.reduce((total, width) => total + width, 0) + Math.max(columnWidths.length - 1, 0) * orgchartColumnGap + orgchartColumnEdgePadding(columnWidths, 'end');
}

export function orgchartDistributedColumnSpace(widths: number[], width: number): number {
	if (widths.length <= 1) return 0;
	const totalColumnWidth = widths.reduce((total, columnWidth) => total + columnWidth, 0);
	const availableWidth = width - orgchartColumnEdgePadding(widths, 'start') - orgchartColumnEdgePadding(widths, 'end');
	return Math.max(orgchartColumnGap, (availableWidth - totalColumnWidth) / (widths.length - 1));
}

export function orgchartColumnCenterPositions(widths: number[], gap: number, width: number): number[] {
	return orgchartColumnLeftPositions(widths, gap, width).map((left, index) => left + widths[index] / 2);
}

export function orgchartColumnLeftPositions(widths: number[], gap: number, width: number): number[] {
	if (widths.length === 0) return [];
	if (widths.length === 1) return [(width - widths[0]) / 2];
	let currentLeft = orgchartColumnEdgePadding(widths, 'start');
	return widths.map((columnWidth) => {
		const left = currentLeft;
		currentLeft += columnWidth + gap;
		return left;
	});
}

export function orgchartRootCenterPositions(rootCount: number, width: number): number[] {
	if (rootCount <= 0) return [];
	const rootRowWidth = orgchartRootRowMinimumWidth(rootCount);
	const left = (width - rootRowWidth) / 2;
	return Array.from({ length: rootCount }, (_, index) => left + index * (orgchartRootNodeWidth + orgchartSiblingGap) + orgchartRootNodeWidth / 2);
}

export function orgchartRootLayouts(roots: UserRecord[], layouts: OrgchartTeamLayout[], columnLefts: number[], width: number): OrgchartPositionedRoot[] {
	const fallbackCenters = orgchartRootCenterPositions(roots.length, width);
	const targetsBySupervisorID = rootTargetsBySupervisorID(layouts, columnLefts);
	const centerCandidates = roots.map((root, index) => {
		const targets = targetsBySupervisorID.get(root.userID) ?? [];
		const fallbackCenter = fallbackCenters[index] ?? width / 2;
		return {
			index,
			center: rootDesiredCenterX(targets, fallbackCenter),
			fallbackCenter,
			hasTargets: targets.length > 0
		};
	});
	const centerPositions = adjustedRootCenterCandidates(centerCandidates, width);
	return roots.map((root, index) => {
		const centerX = centerPositions[index] ?? width / 2;
		return {
			record: root,
			x: centerX - orgchartRootNodeWidth / 2,
			centerX
		};
	});
}

export function orgchartRootConnectorEdges(roots: UserRecord[], layouts: OrgchartTeamLayout[], columnLefts: number[], width: number): OrgchartPositionedEdge[] {
	const rootCentersByUserID = new Map(orgchartRootLayouts(roots, layouts, columnLefts, width).map((root) => [root.record.userID, root.centerX]));
	const rootTargetEntries = [...rootTargetsBySupervisorID(layouts, columnLefts).entries()];
	return rootTargetEntries.flatMap(([supervisorID, targets], index) => {
		const fromX = rootCentersByUserID.get(supervisorID);
		if (fromX === undefined) return [];
		const branchY = rootConnectorMiddleY(index, rootTargetEntries.length);
		return rootConnectorEdges(fromX, branchY, targets);
	});
}

export function orgchartFitZoom(input: OrgchartFitZoomInput): number {
	const minimumZoom = input.minimumZoom ?? orgchartMinimumZoom;
	const maximumZoom = input.maximumZoom ?? orgchartDefaultZoom;
	if (input.boardWidth <= 0 || input.boardHeight <= 0 || input.viewportWidth <= 0 || input.viewportHeight <= 0) return maximumZoom;
	const widthZoom = Math.floor((input.viewportWidth / input.boardWidth) * 100);
	const heightZoom = Math.floor((input.viewportHeight / input.boardHeight) * 100);
	return Math.max(minimumZoom, Math.min(maximumZoom, widthZoom, heightZoom));
}

export function orgchartOrderedTeamLayouts(roots: UserRecord[], layouts: OrgchartTeamLayout[]): OrgchartTeamLayout[] {
	const rootOrderByUserID = new Map(roots.map((root, index) => [root.userID, index]));
	return layouts
		.map((layout, index) => ({ layout, index, rootOrder: layoutRootOrder(layout, rootOrderByUserID) }))
		.sort((first, second) => first.rootOrder - second.rootOrder || first.index - second.index)
		.map((entry) => entry.layout);
}

export function orgchartTeamLayout(column: OrgchartTeamColumn): OrgchartTeamLayout {
	const measuredRoots = column.treeRoots.map(measureTree);
	const forestWidth = measuredForestWidth(measuredRoots);
	const width = Math.max(orgchartPersonNodeWidth, forestWidth);
	const headerWidth = column.treeRoots.length > 1 ? width : orgchartPersonNodeWidth;
	const nodes: OrgchartPositionedNode[] = [];
	const edges: OrgchartPositionedEdge[] = [];
	const rootTop = column.treeRoots.length === 1 ? orgchartTeamHeaderHeight : orgchartTeamHeaderHeight + orgchartLevelGap;
	let rootLeft = (width - forestWidth) / 2;
	for (const root of measuredRoots) {
		const rootCenterX = rootLeft + root.width / 2;
		if (column.treeRoots.length > 1) {
			edges.push({
				fromX: width / 2,
				fromY: orgchartTeamHeaderHeight,
				toX: rootCenterX,
				toY: rootTop
			});
		}
		placeTree(root, rootLeft, rootTop, column.treeRoots.length === 1, nodes, edges);
		rootLeft += root.width + orgchartSiblingGap;
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

export function orgchartEdgePath(edge: OrgchartPositionedEdge): string {
	if (edge.fromX === edge.toX) return `M ${edge.fromX} ${edge.fromY} V ${edge.toY}`;
	if (edge.fromY === edge.toY) return `M ${edge.fromX} ${edge.fromY} H ${edge.toX}`;
	const middleY = edge.middleY ?? edge.fromY + (edge.toY - edge.fromY) / 2;
	return `M ${edge.fromX} ${edge.fromY} V ${middleY} H ${edge.toX} V ${edge.toY}`;
}

function rootTargetsBySupervisorID(layouts: OrgchartTeamLayout[], columnLefts: number[]): Map<string, OrgchartRootTarget[]> {
	const targetsBySupervisorID = new Map<string, OrgchartRootTarget[]>();
	for (const [index, layout] of layouts.entries()) {
		const columnLeft = columnLefts[index] ?? 0;
		const rootTargets = layout.column.treeRoots.flatMap((root) => {
			const supervisorID = root.record.supervisorID?.trim() ?? '';
			const node = layout.nodes.find((candidate) => candidate.record.userID === root.record.userID);
			if (!supervisorID || !node) return [];
			return [{
				supervisorID,
				x: columnLeft + node.x + orgchartPersonNodeWidth / 2,
				left: columnLeft + node.x,
				right: columnLeft + node.x + orgchartPersonNodeWidth
			}];
		});
		const supervisorIDs = new Set(rootTargets.map((target) => target.supervisorID));
		if (supervisorIDs.size === 1 && rootTargets[0]) {
			const target = rootTargets[0];
			addRootTarget(targetsBySupervisorID, target.supervisorID, {
				supervisorID: target.supervisorID,
				x: columnLeft + layout.width / 2,
				left: columnLeft,
				right: columnLeft + layout.width
			});
			continue;
		}
		for (const target of rootTargets) {
			addRootTarget(targetsBySupervisorID, target.supervisorID, target);
		}
	}
	return targetsBySupervisorID;
}

function addRootTarget(targetsBySupervisorID: Map<string, OrgchartRootTarget[]>, supervisorID: string, target: OrgchartRootTarget): void {
	targetsBySupervisorID.set(supervisorID, [...(targetsBySupervisorID.get(supervisorID) ?? []), target]);
}

function rootDesiredCenterX(targets: OrgchartRootTarget[], fallback: number): number {
	if (targets.length === 0) return fallback;
	return (Math.min(...targets.map((target) => target.left)) + Math.max(...targets.map((target) => target.right))) / 2;
}

function adjustedRootCenterCandidates(candidates: OrgchartRootCenterCandidate[], width: number): number[] {
	if (candidates.length === 0) return [];
	const centers = candidates.map((candidate) => candidate.fallbackCenter);
	const targetCandidates = candidates.filter((candidate) => candidate.hasTargets);
	const targetCenters = adjustedRootCenterPositions(targetCandidates.map((candidate) => candidate.center), width);
	const occupiedCenters: number[] = [];
	for (const [index, candidate] of targetCandidates.entries()) {
		const center = targetCenters[index] ?? candidate.center;
		centers[candidate.index] = center;
		occupiedCenters.push(center);
	}
	const fallbackSlots = orgchartRootCenterPositions(candidates.length, width);
	for (const candidate of candidates.filter((entry) => !entry.hasTargets)) {
		const fallbackCenter = fallbackSlots[candidate.index] ?? candidate.fallbackCenter;
		const center = closestAvailableRootCenter(fallbackCenter, fallbackSlots, occupiedCenters, width);
		centers[candidate.index] = center;
		occupiedCenters.push(center);
	}
	return centers;
}

function adjustedRootCenterPositions(desiredCenters: number[], width: number): number[] {
	if (desiredCenters.length === 0) return [];
	const minimumCenter = orgchartRootNodeWidth / 2;
	const maximumCenter = width - orgchartRootNodeWidth / 2;
	const minimumGap = rootCenterMinimumGap();
	const positionedCenters = desiredCenters
		.map((center, index) => ({ center: Math.min(Math.max(center, minimumCenter), maximumCenter), index }))
		.sort((first, second) => first.center - second.center || first.index - second.index);
	for (let index = 1; index < positionedCenters.length; index += 1) {
		positionedCenters[index].center = Math.max(positionedCenters[index].center, positionedCenters[index - 1].center + minimumGap);
	}
	const overflow = positionedCenters[positionedCenters.length - 1].center - maximumCenter;
	if (overflow > 0) {
		for (const positionedCenter of positionedCenters) {
			positionedCenter.center -= overflow;
		}
	}
	for (let index = positionedCenters.length - 2; index >= 0; index -= 1) {
		positionedCenters[index].center = Math.min(positionedCenters[index].center, positionedCenters[index + 1].center - minimumGap);
	}
	const centers = Array.from({ length: desiredCenters.length }, () => width / 2);
	for (const positionedCenter of positionedCenters) {
		centers[positionedCenter.index] = positionedCenter.center;
	}
	return centers;
}

function closestAvailableRootCenter(fallbackCenter: number, fallbackSlots: number[], occupiedCenters: number[], width: number): number {
	const candidates = uniqueRootCenters([fallbackCenter, ...fallbackSlots].map((center) => clampRootCenter(center, width)))
		.filter((center) => isRootCenterAvailable(center, occupiedCenters))
		.sort((first, second) => Math.abs(first - fallbackCenter) - Math.abs(second - fallbackCenter) || first - second);
	return candidates[0] ?? nearestAvailableRootCenter(fallbackCenter, occupiedCenters, width);
}

function nearestAvailableRootCenter(fallbackCenter: number, occupiedCenters: number[], width: number): number {
	const clampedFallbackCenter = clampRootCenter(fallbackCenter, width);
	const minimumGap = rootCenterMinimumGap();
	for (let step = 0; step <= occupiedCenters.length + 2; step += 1) {
		const offset = minimumGap * step;
		const candidates = uniqueRootCenters([
			clampRootCenter(clampedFallbackCenter - offset, width),
			clampRootCenter(clampedFallbackCenter + offset, width)
		]).sort((first, second) => Math.abs(first - fallbackCenter) - Math.abs(second - fallbackCenter) || first - second);
		const availableCenter = candidates.find((center) => isRootCenterAvailable(center, occupiedCenters));
		if (availableCenter !== undefined) return availableCenter;
	}
	return clampedFallbackCenter;
}

function uniqueRootCenters(centers: number[]): number[] {
	return [...new Set(centers)];
}

function isRootCenterAvailable(center: number, occupiedCenters: number[]): boolean {
	const minimumGap = rootCenterMinimumGap();
	return occupiedCenters.every((occupiedCenter) => Math.abs(center - occupiedCenter) >= minimumGap);
}

function clampRootCenter(center: number, width: number): number {
	const minimumCenter = orgchartRootNodeWidth / 2;
	const maximumCenter = width - orgchartRootNodeWidth / 2;
	return Math.min(Math.max(center, minimumCenter), maximumCenter);
}

function rootCenterMinimumGap(): number {
	return orgchartRootNodeWidth + Math.min(orgchartColumnGap, orgchartSiblingGap);
}

function orgchartColumnEdgePadding(widths: number[], edge: 'start' | 'end'): number {
	const width = edge === 'start' ? widths[0] : widths[widths.length - 1];
	return Math.max((orgchartRootNodeWidth - width) / 2, 0);
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

function layoutRootOrder(layout: OrgchartTeamLayout, rootOrderByUserID: Map<string, number>): number {
	const rootOrders = layout.column.treeRoots.flatMap((root) => {
		const supervisorID = root.record.supervisorID?.trim() ?? '';
		const rootOrder = rootOrderByUserID.get(supervisorID);
		return rootOrder === undefined ? [] : [rootOrder];
	});
	if (rootOrders.length === 0) return Number.MAX_SAFE_INTEGER;
	return Math.min(...rootOrders);
}

function measureTree(node: OrgchartTreeNode): MeasuredTree {
	const children = node.children.map(measureTree);
	return {
		node,
		width: Math.max(orgchartPersonNodeWidth, measuredForestWidth(children)),
		children
	};
}

function measuredForestWidth(trees: MeasuredTree[]): number {
	if (trees.length === 0) return orgchartPersonNodeWidth;
	return trees.reduce((total, tree) => total + tree.width, 0) + Math.max(trees.length - 1, 0) * orgchartSiblingGap;
}

function placeTree(
	tree: MeasuredTree,
	left: number,
	top: number,
	isAttachedToHeader: boolean,
	nodes: OrgchartPositionedNode[],
	edges: OrgchartPositionedEdge[]
): void {
	const centerX = left + tree.width / 2;
	nodes.push({
		record: tree.node.record,
		x: centerX - orgchartPersonNodeWidth / 2,
		y: top,
		isAttachedToHeader
	});
	if (tree.children.length === 0) return;
	const childTop = top + orgchartPersonNodeHeight + orgchartLevelGap;
	let childLeft = left + (tree.width - measuredForestWidth(tree.children)) / 2;
	for (const child of tree.children) {
		const childCenterX = childLeft + child.width / 2;
		edges.push({
			fromX: centerX,
			fromY: top + orgchartPersonNodeHeight,
			toX: childCenterX,
			toY: childTop
		});
		placeTree(child, childLeft, childTop, false, nodes, edges);
		childLeft += child.width + orgchartSiblingGap;
	}
}
