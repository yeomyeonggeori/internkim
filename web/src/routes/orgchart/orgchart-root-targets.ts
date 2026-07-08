import {
	orgchartPersonNodeWidth
} from './orgchart-layout-constants';
import type { OrgchartRootTarget, OrgchartTeamLayout } from './orgchart-layout-types';

export function rootTargetsBySupervisorID(layouts: OrgchartTeamLayout[], columnLefts: number[]): Map<string, OrgchartRootTarget[]> {
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
		for (const targets of rootTargetsBySupervisor(rootTargets).values()) {
			const target = rootGroupTarget(targets);
			addRootTarget(targetsBySupervisorID, target.supervisorID, target);
		}
	}
	return targetsBySupervisorID;
}

export function layoutRootSupervisorIDs(layout: OrgchartTeamLayout): string[] {
	return Array.from(new Set(layout.column.treeRoots.map((root) => root.record.supervisorID?.trim() ?? '').filter(Boolean)));
}

function rootTargetsBySupervisor(targets: OrgchartRootTarget[]): Map<string, OrgchartRootTarget[]> {
	const targetsBySupervisor = new Map<string, OrgchartRootTarget[]>();
	for (const target of targets) {
		targetsBySupervisor.set(target.supervisorID, [...(targetsBySupervisor.get(target.supervisorID) ?? []), target]);
	}
	return targetsBySupervisor;
}

function rootGroupTarget(targets: OrgchartRootTarget[]): OrgchartRootTarget {
	const firstTarget = targets[0];
	const left = Math.min(...targets.map((target) => target.left));
	const right = Math.max(...targets.map((target) => target.right));
	return {
		supervisorID: firstTarget.supervisorID,
		x: (left + right) / 2,
		left,
		right
	};
}

function addRootTarget(targetsBySupervisorID: Map<string, OrgchartRootTarget[]>, supervisorID: string, target: OrgchartRootTarget): void {
	targetsBySupervisorID.set(supervisorID, [...(targetsBySupervisorID.get(supervisorID) ?? []), target]);
}
