import {
	type OrgchartRootCenterCandidate,
	orgchartRootCenterPositions
} from './orgchart-root-center-candidates';
import {
	adjustedRootCenterPositions,
	adjustedRootCenterPositionsInDisplayOrder,
	closestAvailableRootCenter
} from './orgchart-root-center-spacing';

export function adjustedRootCenterCandidates(candidates: OrgchartRootCenterCandidate[], width: number): number[] {
	if (candidates.length === 0) return [];
	const centers = candidates.map((candidate) => candidate.fallbackCenter);
	const targetCandidates = candidates.filter((candidate) => candidate.hasTargets);
	const targetCenters = adjustedRootCenterPositions(targetCandidates.map((candidate) => candidate.center), width);
	const occupiedCenters: number[] = [];
	let needsGlobalAdjustment = false;
	for (const [index, candidate] of targetCandidates.entries()) {
		const center = targetCenters[index] ?? candidate.center;
		centers[candidate.index] = center;
		occupiedCenters.push(center);
	}
	const fallbackSlots = orgchartRootCenterPositions(candidates.length, width);
	for (const candidate of candidates.filter((entry) => !entry.hasTargets)) {
		const fallbackCenter = fallbackSlots[candidate.index] ?? candidate.fallbackCenter;
		const center = closestAvailableRootCenter(fallbackCenter, fallbackSlots, occupiedCenters, width);
		if (center === undefined) {
			centers[candidate.index] = fallbackCenter;
			needsGlobalAdjustment = true;
			continue;
		}
		centers[candidate.index] = center;
		occupiedCenters.push(center);
	}
	if (needsGlobalAdjustment) {
		return adjustedRootCenterPositionsInDisplayOrder(candidates.map((candidate) => (candidate.hasTargets ? candidate.center : candidate.fallbackCenter)), width);
	}
	return centers;
}
