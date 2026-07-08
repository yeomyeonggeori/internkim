import {
	orgchartColumnGap,
	orgchartDefaultZoom,
	orgchartMinimumZoom,
	orgchartRootNodeWidth
} from './orgchart-layout-constants';
import type { OrgchartFitZoomInput, OrgchartPositionedEdge } from './orgchart-layout-types';

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

export function orgchartFitZoom(input: OrgchartFitZoomInput): number {
	const minimumZoom = input.minimumZoom ?? orgchartMinimumZoom;
	const maximumZoom = input.maximumZoom ?? orgchartDefaultZoom;
	if (input.boardWidth <= 0 || input.boardHeight <= 0 || input.viewportWidth <= 0 || input.viewportHeight <= 0) return maximumZoom;
	const widthZoom = Math.floor((input.viewportWidth / input.boardWidth) * 100);
	const heightZoom = Math.floor((input.viewportHeight / input.boardHeight) * 100);
	return Math.max(minimumZoom, Math.min(maximumZoom, widthZoom, heightZoom));
}

export function orgchartEdgePath(edge: OrgchartPositionedEdge): string {
	if (edge.fromX === edge.toX) return `M ${edge.fromX} ${edge.fromY} V ${edge.toY}`;
	if (edge.fromY === edge.toY) return `M ${edge.fromX} ${edge.fromY} H ${edge.toX}`;
	const middleY = edge.middleY ?? edge.fromY + (edge.toY - edge.fromY) / 2;
	return `M ${edge.fromX} ${edge.fromY} V ${middleY} H ${edge.toX} V ${edge.toY}`;
}

function orgchartColumnEdgePadding(widths: number[], edge: 'start' | 'end'): number {
	const width = edge === 'start' ? widths[0] : widths[widths.length - 1];
	return Math.max((orgchartRootNodeWidth - width) / 2, 0);
}
