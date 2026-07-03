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
};

export type OrgchartTeamLayout = {
	column: OrgchartTeamColumn;
	width: number;
	height: number;
	headerX: number;
	nodes: OrgchartPositionedNode[];
	edges: OrgchartPositionedEdge[];
};

export const orgchartPersonNodeWidth = 220;
export const orgchartPersonNodeHeight = 96;
export const orgchartTeamHeaderHeight = 56;
export const orgchartColumnGap = 28;
export const orgchartSiblingGap = 16;
export const orgchartLevelGap = 32;
export const orgchartColumnPaddingBottom = 4;

export function orgchartBoardMinimumWidth(columnWidths: number[]): number {
	return columnWidths.reduce((total, width) => total + width, 0) + Math.max(columnWidths.length - 1, 0) * orgchartColumnGap;
}

export function orgchartDistributedColumnSpace(widths: number[], width: number): number {
	if (widths.length <= 1) return 0;
	const totalColumnWidth = widths.reduce((total, columnWidth) => total + columnWidth, 0);
	return Math.max(orgchartColumnGap, (width - totalColumnWidth) / (widths.length - 1));
}

export function orgchartColumnCenterPositions(widths: number[], gap: number, width: number): number[] {
	if (widths.length === 1) return [width / 2];
	let currentLeft = 0;
	return widths.map((columnWidth) => {
		const center = currentLeft + columnWidth / 2;
		currentLeft += columnWidth + gap;
		return center;
	});
}

export function orgchartTeamLayout(column: OrgchartTeamColumn): OrgchartTeamLayout {
	const measuredRoots = column.treeRoots.map(measureTree);
	const forestWidth = measuredForestWidth(measuredRoots);
	const width = Math.max(orgchartPersonNodeWidth, forestWidth);
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
		headerX: (width - orgchartPersonNodeWidth) / 2,
		nodes,
		edges
	};
}

export function orgchartEdgePath(edge: OrgchartPositionedEdge): string {
	const middleY = edge.fromY + (edge.toY - edge.fromY) / 2;
	return `M ${edge.fromX} ${edge.fromY} V ${middleY} H ${edge.toX} V ${edge.toY}`;
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
