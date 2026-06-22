export type TimelineEventLanePlacement = {
	eventID: string;
	kind: 'lane' | 'layer';
	leftPercent: number;
	widthPercent: number;
	zIndex: number;
};

export type TimelineEventLaneCandidate = {
	eventID: string;
	title: string;
	startDate: Date;
	endDate: Date;
};

const laneGapPercent = 1.2;
const nestedLayerInsetPercent = 2;
const labelCollisionMinutes = 45;
const backgroundLayerZIndex = 1010;
const laneLayerZIndex = 1020;
const nestedLayerZIndex = 1030;

export function timelineEventLanePlacementsFromCandidates(
	candidates: TimelineEventLaneCandidate[]
): TimelineEventLanePlacement[] {
	return timelineEventLaneClusters(candidates).flatMap(timelineEventLanePlacementsFromCluster);
}

function timelineEventLaneClusters(candidates: TimelineEventLaneCandidate[]): TimelineEventLaneCandidate[][] {
	const clusters: TimelineEventLaneCandidate[][] = [];
	let currentCluster: TimelineEventLaneCandidate[] = [];
	let currentClusterEndDate: Date | null = null;
	for (const candidate of candidates) {
		if (!currentClusterEndDate || candidate.startDate < currentClusterEndDate) {
			currentCluster.push(candidate);
			currentClusterEndDate = laterDate(currentClusterEndDate, candidate.endDate);
			continue;
		}
		clusters.push(currentCluster);
		currentCluster = [candidate];
		currentClusterEndDate = candidate.endDate;
	}
	if (currentCluster.length > 0) clusters.push(currentCluster);
	return clusters;
}

function timelineEventLanePlacementsFromCluster(cluster: TimelineEventLaneCandidate[]): TimelineEventLanePlacement[] {
	if (cluster.length <= 1) return [];
	const placements = new Map<string, TimelineEventLanePlacement>();
	for (const labelCollisionGroup of labelCollisionGroups(cluster)) {
		if (labelCollisionGroup.length <= 1) continue;
		for (const placement of lanePlacementsFromGroup(labelCollisionGroup)) {
			placements.set(placement.eventID, placement);
		}
	}
	for (const candidate of cluster) {
		if (placements.has(candidate.eventID)) continue;
		const parentPlacement = containingParentPlacement(candidate, cluster, placements);
		placements.set(candidate.eventID, layerPlacement(candidate, parentPlacement));
	}
	return Array.from(placements.values());
}

function labelCollisionGroups(cluster: TimelineEventLaneCandidate[]): TimelineEventLaneCandidate[][] {
	const groups: TimelineEventLaneCandidate[][] = [];
	const visitedEventIDs = new Set<string>();
	for (const candidate of cluster) {
		if (visitedEventIDs.has(candidate.eventID)) continue;
		const group = collectLabelCollisionGroup(candidate, cluster, visitedEventIDs);
		groups.push(group);
	}
	return groups;
}

function collectLabelCollisionGroup(
	initialCandidate: TimelineEventLaneCandidate,
	cluster: TimelineEventLaneCandidate[],
	visitedEventIDs: Set<string>
): TimelineEventLaneCandidate[] {
	const queue = [initialCandidate];
	const group: TimelineEventLaneCandidate[] = [];
	visitedEventIDs.add(initialCandidate.eventID);
	while (queue.length > 0) {
		const candidate = queue.shift();
		if (!candidate) continue;
		group.push(candidate);
		for (const otherCandidate of cluster) {
			if (visitedEventIDs.has(otherCandidate.eventID)) continue;
			if (!hasLabelCollision(candidate, otherCandidate)) continue;
			visitedEventIDs.add(otherCandidate.eventID);
			queue.push(otherCandidate);
		}
	}
	return group.sort(
		(firstEvent, secondEvent) =>
			firstEvent.startDate.getTime() - secondEvent.startDate.getTime() ||
			secondEvent.endDate.getTime() - firstEvent.endDate.getTime()
	);
}

function lanePlacementsFromGroup(group: TimelineEventLaneCandidate[]): TimelineEventLanePlacement[] {
	const laneEndDates: Date[] = [];
	const placements = group.map((event) => {
		const laneIndex = firstAvailableLaneIndex(laneEndDates, event.startDate);
		laneEndDates[laneIndex] = event.endDate;
		return {
			eventID: event.eventID,
			kind: 'lane' as const,
			leftPercent: laneIndex * 100,
			widthPercent: 100,
			zIndex: laneLayerZIndex,
			laneIndex
		};
	});
	const laneCount = laneEndDates.length;
	return placements.map((placement) => ({
		eventID: placement.eventID,
		kind: placement.kind,
		leftPercent: (placement.laneIndex * 100) / laneCount,
		widthPercent: Math.max(0, 100 / laneCount - laneGapPercent),
		zIndex: placement.zIndex
	}));
}

function firstAvailableLaneIndex(laneEndDates: Date[], startDate: Date): number {
	const laneIndex = laneEndDates.findIndex((laneEndDate) => laneEndDate <= startDate);
	return laneIndex >= 0 ? laneIndex : laneEndDates.length;
}

function layerPlacement(
	candidate: TimelineEventLaneCandidate,
	parentPlacement: TimelineEventLanePlacement | null
): TimelineEventLanePlacement {
	if (!parentPlacement) {
		return {
			eventID: candidate.eventID,
			kind: 'layer',
			leftPercent: 0,
			widthPercent: 100,
			zIndex: backgroundLayerZIndex
		};
	}
	return {
		eventID: candidate.eventID,
		kind: 'layer',
		leftPercent: parentPlacement.leftPercent + nestedLayerInsetPercent,
		widthPercent: Math.max(0, parentPlacement.widthPercent - nestedLayerInsetPercent * 2),
		zIndex: nestedLayerZIndex
	};
}

function containingParentPlacement(
	candidate: TimelineEventLaneCandidate,
	cluster: TimelineEventLaneCandidate[],
	placements: Map<string, TimelineEventLanePlacement>
): TimelineEventLanePlacement | null {
	const parentCandidate = cluster
		.filter((otherCandidate) => otherCandidate.eventID !== candidate.eventID)
		.filter((otherCandidate) => containsEvent(otherCandidate, candidate))
		.filter((otherCandidate) => placements.has(otherCandidate.eventID))
		.sort(parentCandidateOrder)[0];
	if (!parentCandidate) return null;
	return placements.get(parentCandidate.eventID) ?? null;
}

function parentCandidateOrder(firstCandidate: TimelineEventLaneCandidate, secondCandidate: TimelineEventLaneCandidate): number {
	const firstDuration = firstCandidate.endDate.getTime() - firstCandidate.startDate.getTime();
	const secondDuration = secondCandidate.endDate.getTime() - secondCandidate.startDate.getTime();
	return (
		firstCandidate.startDate.getTime() - secondCandidate.startDate.getTime() ||
		secondDuration - firstDuration ||
		firstCandidate.eventID.localeCompare(secondCandidate.eventID)
	);
}

function hasLabelCollision(firstCandidate: TimelineEventLaneCandidate, secondCandidate: TimelineEventLaneCandidate): boolean {
	if (!eventsOverlap(firstCandidate, secondCandidate)) return false;
	if (hasSameTitle(firstCandidate, secondCandidate)) return true;
	return Math.abs(minutesBetween(firstCandidate.startDate, secondCandidate.startDate)) < labelCollisionMinutes;
}

function hasSameTitle(firstCandidate: TimelineEventLaneCandidate, secondCandidate: TimelineEventLaneCandidate): boolean {
	const firstTitle = normalizedTitle(firstCandidate.title);
	const secondTitle = normalizedTitle(secondCandidate.title);
	return firstTitle.length > 0 && firstTitle === secondTitle;
}

function normalizedTitle(title: string): string {
	return title.trim().replace(/\s+/g, ' ').toLocaleLowerCase();
}

function eventsOverlap(firstCandidate: TimelineEventLaneCandidate, secondCandidate: TimelineEventLaneCandidate): boolean {
	return firstCandidate.startDate < secondCandidate.endDate && secondCandidate.startDate < firstCandidate.endDate;
}

function containsEvent(parentCandidate: TimelineEventLaneCandidate, childCandidate: TimelineEventLaneCandidate): boolean {
	return parentCandidate.startDate <= childCandidate.startDate && childCandidate.endDate <= parentCandidate.endDate;
}

function minutesBetween(firstDate: Date, secondDate: Date): number {
	return (firstDate.getTime() - secondDate.getTime()) / 60000;
}

function laterDate(firstDate: Date | null, secondDate: Date): Date {
	if (!firstDate || secondDate > firstDate) return secondDate;
	return firstDate;
}
