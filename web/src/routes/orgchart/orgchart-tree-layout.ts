import type { OrgchartTreeNode } from './orgchart-directory-model';
import {
	orgchartLevelGap,
	orgchartPersonNodeHeight,
	orgchartPersonNodeWidth,
	orgchartSiblingGap
} from './orgchart-layout-constants';
import type { OrgchartPositionedEdge, OrgchartPositionedNode } from './orgchart-layout-types';

export type MeasuredTree = {
	node: OrgchartTreeNode;
	width: number;
	children: MeasuredTree[];
};

export function measureTree(node: OrgchartTreeNode): MeasuredTree {
	const children = node.children.map(measureTree);
	return {
		node,
		width: Math.max(orgchartPersonNodeWidth, measuredForestWidth(children)),
		children
	};
}

export function measuredForestWidth(trees: MeasuredTree[]): number {
	if (trees.length === 0) return orgchartPersonNodeWidth;
	return trees.reduce((total, tree) => total + tree.width, 0) + Math.max(trees.length - 1, 0) * orgchartSiblingGap;
}

export function placeTree(
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
