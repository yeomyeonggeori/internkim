import type { AttendanceEvent } from '../attendance-context.svelte';

export type AttendanceWorkSegment = {
	id: string;
	date: string;
	clockIn: AttendanceEvent;
	clockOut?: AttendanceEvent;
	endEvent?: AttendanceEvent;
	endReason?: 'clock_out' | 'next_clock_in';
	locationID?: string;
	locationName?: string;
	startTime: string;
	endTime?: string;
	workedMinutes: number;
	isOpen: boolean;
};

export function buildAttendanceWorkSegments(date: string, events: AttendanceEvent[]): AttendanceWorkSegment[] {
	const sortedEvents = events
		.filter((event) => !event.canceledAt && event.localDate === date)
		.sort((a, b) => a.occurredAt.localeCompare(b.occurredAt));
	const segments: AttendanceWorkSegment[] = [];
	let openClockIn: AttendanceEvent | undefined;
	for (const event of sortedEvents) {
		if (event.kind === 'clock_in') {
			if (openClockIn) segments.push(createAttendanceWorkSegment(date, openClockIn, event, 'next_clock_in'));
			openClockIn = event;
			continue;
		}
		if (!openClockIn) continue;
		segments.push(createAttendanceWorkSegment(date, openClockIn, event, 'clock_out'));
		openClockIn = undefined;
	}
	if (openClockIn) segments.push(createAttendanceWorkSegment(date, openClockIn));
	return segments;
}

function createAttendanceWorkSegment(
	date: string,
	clockIn: AttendanceEvent,
	endEvent?: AttendanceEvent,
	endReason?: AttendanceWorkSegment['endReason']
): AttendanceWorkSegment {
	const clockOut = endReason === 'clock_out' ? endEvent : undefined;
	return {
		id: `${clockIn.id}-${endEvent?.id ?? 'open'}`,
		date,
		clockIn,
		clockOut,
		endEvent,
		endReason,
		locationID: clockIn.locationID,
		locationName: clockIn.locationName,
		startTime: clockIn.localTime,
		endTime: endEvent?.localTime,
		workedMinutes: endEvent ? minutesBetween(clockIn.occurredAt, endEvent.occurredAt) : 0,
		isOpen: !endEvent,
	};
}

function minutesBetween(start: string, end: string): number {
	const startMs = new Date(start).getTime();
	const endMs = new Date(end).getTime();
	if (Number.isNaN(startMs) || Number.isNaN(endMs) || endMs <= startMs) return 0;
	return Math.round((endMs - startMs) / 60000);
}
