import {
	orgchartRootNodeWidth,
	orgchartSiblingGap
} from './orgchart-layout-constants';
import type { OrgchartRootTarget } from './orgchart-layout-types';

export type OrgchartRootCenterCandidate = {
	index: number;
	center: number;
	fallbackCenter: number;
	hasTargets: boolean;
};

export function rootDesiredCenterX(targets: OrgchartRootTarget[], fallback: number): number {
	if (targets.length === 0) return fallback;
	return (Math.min(...targets.map((target) => target.left)) + Math.max(...targets.map((target) => target.right))) / 2;
}

export function orgchartRootCenterPositions(rootCount: number, width: number): number[] {
	if (rootCount <= 0) return [];
	const rootRowWidth = orgchartRootRowMinimumWidth(rootCount);
	const left = (width - rootRowWidth) / 2;
	return Array.from({ length: rootCount }, (_, index) => left + index * (orgchartRootNodeWidth + orgchartSiblingGap) + orgchartRootNodeWidth / 2);
}

export function orgchartRootRowMinimumWidth(rootCount: number): number {
	if (rootCount <= 0) return 0;
	return rootCount * orgchartRootNodeWidth + (rootCount - 1) * orgchartSiblingGap;
}
