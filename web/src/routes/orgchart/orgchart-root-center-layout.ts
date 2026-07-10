import type { UserRecord } from '../admin/admin-types';
import { orgchartRootNodeWidth } from './orgchart-layout-constants';
import type { OrgchartPositionedRoot, OrgchartTeamLayout } from './orgchart-layout-types';
import { adjustedRootCenterCandidates } from './orgchart-root-center-adjustment';
import { orgchartRootCenterPositions, orgchartRootRowMinimumWidth, rootDesiredCenterX } from './orgchart-root-center-candidates';
import { rootTargetsBySupervisorID } from './orgchart-root-targets';

export { orgchartRootCenterPositions, orgchartRootRowMinimumWidth };

export function orgchartRootLayouts(
	roots: UserRecord[],
	layouts: OrgchartTeamLayout[],
	columnLefts: number[],
	width: number,
	rootCentersByUserID?: Map<string, number>
): OrgchartPositionedRoot[] {
	if (rootCentersByUserID) {
		return roots.map((root) => {
			const centerX = rootCentersByUserID.get(root.userID) ?? width / 2;
			return {
				record: root,
				x: centerX - orgchartRootNodeWidth / 2,
				centerX
			};
		});
	}
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
