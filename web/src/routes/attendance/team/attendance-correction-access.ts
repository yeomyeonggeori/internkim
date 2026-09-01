import {
	attendanceWriteOutcome,
	combineAttendanceWriteOutcomes,
	type AttendanceWriteOutcome
} from '$lib/attendance/attendance-write';
import type { AttendanceEvent, AttendanceSummary } from '../attendance-context.svelte';
import { attendanceInstantOfLocalTime } from '../shared/attendance-date';
import type { TeamStatusPersonDaySegment } from './team-status-table-model';

export function attendanceEventWriteOutcome(
	summary: AttendanceSummary,
	event: AttendanceEvent,
	currentTime: Date
): AttendanceWriteOutcome {
	return attendanceWriteOutcome(
		summary,
		{
			subjectEmail: event.email,
			anchorTime: new Date(event.originalOccurredAt ?? event.occurredAt)
		},
		currentTime
	);
}

export function attendanceAdditionWriteOutcome(
	summary: AttendanceSummary,
	addition: { email: string; localDate: string; localTime: string },
	currentTime: Date
): AttendanceWriteOutcome {
	return attendanceWriteOutcome(
		summary,
		{
			subjectEmail: addition.email,
			anchorTime: attendanceInstantOfLocalTime(
				addition.localDate,
				addition.localTime,
				summary.timeZone
			)
		},
		currentTime
	);
}

export function attendanceEventsWriteOutcome(
	summary: AttendanceSummary,
	eventIDs: Iterable<string>,
	currentTime: Date
): AttendanceWriteOutcome {
	const eventsByID = new Map(summary.events.map((event) => [event.id, event]));
	return combineAttendanceWriteOutcomes(
		[...eventIDs].map((eventID) => {
			const event = eventsByID.get(eventID);
			if (!event) return 'blocked';
			return attendanceEventWriteOutcome(summary, event, currentTime);
		})
	);
}

export function editableAttendanceEventIDs(
	summary: AttendanceSummary,
	segments: TeamStatusPersonDaySegment[],
	currentTime: Date
): Set<string> {
	const eventIDs = eventIDsFor(segments);
	if (!Number.isFinite(currentTime.getTime())) return new Set<string>();

	const eventsByID = new Map(summary.events.map((event) => [event.id, event]));
	return new Set(
		[...eventIDs].filter((eventID) => {
			const event = eventsByID.get(eventID);
			return (
				event !== undefined &&
				attendanceEventWriteOutcome(summary, event, currentTime) !== 'blocked'
			);
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
