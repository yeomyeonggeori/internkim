import type { AttendanceEvent, AttendanceSummary } from '../attendance-context.svelte';
import type { TeamStatusPersonDaySegment } from './team-status-table-model';

export function editableAttendanceEventIDs(
	summary: AttendanceSummary,
	segments: TeamStatusPersonDaySegment[],
	currentTime: Date
): Set<string> {
	const eventIDs = eventIDsFor(segments);
	if (summary.correctionWindowMinutes === undefined) return eventIDs;
	if (!Number.isFinite(currentTime.getTime())) return new Set<string>();

	const eventsByID = new Map(summary.events.map((event) => [event.id, event]));
	return new Set(
		[...eventIDs].filter((eventID) => {
			const event = eventsByID.get(eventID);
			return event !== undefined && canCorrectEvent(summary, event, currentTime);
		})
	);
}

function eventIDsFor(segments: TeamStatusPersonDaySegment[]): Set<string> {
	const eventIDs = new Set<string>();
	for (const segment of segments) {
		eventIDs.add(segment.startEventID);
		if (segment.endEventID) eventIDs.add(segment.endEventID);
	}
	return eventIDs;
}

function canCorrectEvent(summary: AttendanceSummary, event: AttendanceEvent, currentTime: Date): boolean {
	if (summary.isAdmin) return true;
	if (event.email !== summary.currentUserEmail) return false;

	const anchorTime = new Date(event.originalOccurredAt ?? event.occurredAt);
	const elapsedMilliseconds = currentTime.getTime() - anchorTime.getTime();
	const windowMilliseconds = (summary.correctionWindowMinutes ?? 0) * 60_000;
	return Number.isFinite(anchorTime.getTime()) && elapsedMilliseconds >= 0 && elapsedMilliseconds <= windowMilliseconds;
}
