import {
	orgchartColumnGap,
	orgchartRootNodeWidth,
	orgchartSiblingGap
} from './orgchart-layout-constants';

export function adjustedRootCenterPositions(desiredCenters: number[], width: number): number[] {
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
	if (positionedCenters[0].center < minimumCenter) {
		positionedCenters[0].center = minimumCenter;
		for (let index = 1; index < positionedCenters.length; index += 1) {
			positionedCenters[index].center = Math.max(positionedCenters[index].center, positionedCenters[index - 1].center + minimumGap);
		}
	}
	const centers = Array.from({ length: desiredCenters.length }, () => width / 2);
	for (const positionedCenter of positionedCenters) {
		centers[positionedCenter.index] = positionedCenter.center;
	}
	return centers;
}

export function adjustedRootCenterPositionsInDisplayOrder(desiredCenters: number[], width: number): number[] {
	if (desiredCenters.length === 0) return [];
	const minimumCenter = orgchartRootNodeWidth / 2;
	const maximumCenter = width - orgchartRootNodeWidth / 2;
	const minimumGap = rootCenterMinimumGap();
	const centers = desiredCenters.map((center) => clampRootCenter(center, width));
	for (let index = 1; index < centers.length; index += 1) {
		centers[index] = Math.max(centers[index], centers[index - 1] + minimumGap);
	}
	if (centers[centers.length - 1] > maximumCenter) {
		centers[centers.length - 1] = maximumCenter;
		for (let index = centers.length - 2; index >= 0; index -= 1) {
			centers[index] = Math.min(centers[index], centers[index + 1] - minimumGap);
		}
	}
	if (centers[0] < minimumCenter) {
		centers[0] = minimumCenter;
		for (let index = 1; index < centers.length; index += 1) {
			centers[index] = Math.max(centers[index], centers[index - 1] + minimumGap);
		}
	}
	return centers;
}

export function closestAvailableRootCenter(fallbackCenter: number, fallbackSlots: number[], occupiedCenters: number[], width: number): number | undefined {
	const candidates = uniqueRootCenters([fallbackCenter, ...fallbackSlots].map((center) => clampRootCenter(center, width)))
		.filter((center) => isRootCenterAvailable(center, occupiedCenters))
		.sort((first, second) => Math.abs(first - fallbackCenter) - Math.abs(second - fallbackCenter) || first - second);
	return candidates[0] ?? nearestAvailableRootCenter(fallbackCenter, occupiedCenters, width);
}

function nearestAvailableRootCenter(fallbackCenter: number, occupiedCenters: number[], width: number): number | undefined {
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
	return undefined;
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
