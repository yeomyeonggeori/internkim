import type { AttendanceEvent } from '../attendance-context.svelte';
import {
	buildAttendanceWorkSegments,
	type AttendanceWorkSegment,
	type AttendanceWorkSegmentOptions,
} from './attendance-work-segments';

export type DayEvents = {
	date: string;
	events: AttendanceEvent[];
	segments: AttendanceWorkSegment[];
	activeSegment?: AttendanceWorkSegment;
	clockIn?: AttendanceEvent;
	clockOut?: AttendanceEvent;
	workedMinutes: number;
	inProgress: boolean;
};

export type DayEventsOptions = AttendanceWorkSegmentOptions;

export function groupEventsByDay(events: AttendanceEvent[]): Map<string, AttendanceEvent[]> {
	const map = new Map<string, AttendanceEvent[]>();
	for (const event of events) {
		if (event.canceledAt) continue;
		const list = map.get(event.localDate) ?? [];
		list.push(event);
		map.set(event.localDate, list);
	}
	for (const list of map.values()) {
		list.sort((a, b) => a.occurredAt.localeCompare(b.occurredAt));
	}
	return map;
}

export function computeDayEvents(date: string, events: AttendanceEvent[], options: DayEventsOptions = {}): DayEvents {
	const sorted = events
		.filter((event) => !event.canceledAt)
		.sort((a, b) => a.occurredAt.localeCompare(b.occurredAt));
	const segments = buildAttendanceWorkSegments(date, sorted, options);
	const activeSegment = [...segments].reverse().find((segment) => segment.isOpen);
	const clockIn = [...segments].reverse().find((segment) => segment.clockIn)?.clockIn;
	const clockOut = [...segments].reverse().find((segment) => segment.clockOut)?.clockOut;
	const workedMinutes = segments.reduce((total, segment) => total + segment.workedMinutes, 0);
	const inProgress = !!activeSegment;
	return { date, events: sorted, segments, activeSegment, clockIn, clockOut, workedMinutes, inProgress };
}

export function minutesBetween(start: string, end: string): number {
	const startMs = new Date(start).getTime();
	const endMs = new Date(end).getTime();
	if (Number.isNaN(startMs) || Number.isNaN(endMs) || endMs <= startMs) return 0;
	return Math.round((endMs - startMs) / 60000);
}
