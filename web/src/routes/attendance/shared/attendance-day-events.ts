import type { AttendanceEvent } from '../attendance-context.svelte';

export type DayEvents = {
	date: string;
	events: AttendanceEvent[];
	clockIn?: AttendanceEvent;
	clockOut?: AttendanceEvent;
	workedMinutes: number;
	inProgress: boolean;
};

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

export function computeDayEvents(date: string, events: AttendanceEvent[]): DayEvents {
	const sorted = [...events].sort((a, b) => a.occurredAt.localeCompare(b.occurredAt));
	const clockIn = sorted.find((event) => event.kind === 'clock_in');
	const clockOut = [...sorted].reverse().find((event) => event.kind === 'clock_out');
	const workedMinutes = clockIn && clockOut ? minutesBetween(clockIn.occurredAt, clockOut.occurredAt) : 0;
	const inProgress = !!clockIn && !clockOut;
	return { date, events: sorted, clockIn, clockOut, workedMinutes, inProgress };
}

export function minutesBetween(start: string, end: string): number {
	const startMs = new Date(start).getTime();
	const endMs = new Date(end).getTime();
	if (Number.isNaN(startMs) || Number.isNaN(endMs) || endMs <= startMs) return 0;
	return Math.round((endMs - startMs) / 60000);
}
