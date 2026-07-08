import type { UserRecord } from '../admin/admin-types';
import { orgchartBoardMinimumWidth } from './orgchart-board-layout';
import {
	orgchartColumnGap,
	orgchartRootNodeWidth
} from './orgchart-layout-constants';
import type {
	OrgchartRootBlockLayout,
	OrgchartTeamLayout
} from './orgchart-layout-types';
import { orgchartRootRowMinimumWidth, rootDesiredCenterX } from './orgchart-root-center-candidates';
import { layoutRootSupervisorIDs, rootTargetsBySupervisorID } from './orgchart-root-targets';

export function orgchartRootBlockLayout(roots: UserRecord[], layouts: OrgchartTeamLayout[]): OrgchartRootBlockLayout {
	const layoutIndexesByRootID = rootBlockLayoutIndexesByRootID(roots, layouts);
	const targetSupervisorIDs = new Set(rootTargetsBySupervisorID(layouts, Array.from({ length: layouts.length }, () => 0)).keys());
	const columnLefts = Array.from({ length: layouts.length }, () => 0);
	const rootCentersByUserID = new Map<string, number>();
	let currentLeft = 0;
	for (const root of roots) {
		const layoutIndexes = layoutIndexesByRootID.get(root.userID) ?? [];
		if (layoutIndexes.length === 0 && targetSupervisorIDs.has(root.userID)) continue;
		const blockWidth = rootBlockWidth(layoutIndexes, layouts);
		rootCentersByUserID.set(root.userID, currentLeft + blockWidth / 2);
		let layoutLeft = currentLeft;
		for (const layoutIndex of layoutIndexes) {
			columnLefts[layoutIndex] = layoutLeft;
			layoutLeft += layouts[layoutIndex].width + orgchartColumnGap;
		}
		currentLeft += blockWidth + orgchartColumnGap;
	}
	const targetsBySupervisorID = rootTargetsBySupervisorID(layouts, columnLefts);
	for (const root of roots) {
		const targets = targetsBySupervisorID.get(root.userID) ?? [];
		if (targets.length === 0) continue;
		rootCentersByUserID.set(root.userID, rootDesiredCenterX(targets, rootCentersByUserID.get(root.userID) ?? 0));
	}
	return {
		width: Math.max(currentLeft - (roots.length > 0 ? orgchartColumnGap : 0), orgchartBoardMinimumWidth(layouts.map((layout) => layout.width)), orgchartRootRowMinimumWidth(roots.length)),
		columnLefts,
		rootCentersByUserID
	};
}

function rootBlockLayoutIndexesByRootID(roots: UserRecord[], layouts: OrgchartTeamLayout[]): Map<string, number[]> {
	const rootOrderByUserID = new Map(roots.map((root, index) => [root.userID, index]));
	const layoutIndexesByRootID = new Map<string, number[]>();
	for (const [layoutIndex, layout] of layouts.entries()) {
		const supervisorIDs = layoutRootSupervisorIDs(layout)
			.filter((supervisorID) => rootOrderByUserID.has(supervisorID))
			.sort((first, second) => (rootOrderByUserID.get(first) ?? Number.MAX_SAFE_INTEGER) - (rootOrderByUserID.get(second) ?? Number.MAX_SAFE_INTEGER));
		const rootID = supervisorIDs[0];
		if (!rootID) continue;
		layoutIndexesByRootID.set(rootID, [...(layoutIndexesByRootID.get(rootID) ?? []), layoutIndex]);
	}
	return layoutIndexesByRootID;
}

function rootBlockWidth(layoutIndexes: number[], layouts: OrgchartTeamLayout[]): number {
	if (layoutIndexes.length === 0) return orgchartRootNodeWidth;
	return layoutIndexes.reduce((total, layoutIndex) => total + layouts[layoutIndex].width, 0) + Math.max(layoutIndexes.length - 1, 0) * orgchartColumnGap;
}
