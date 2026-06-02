import type { AttendanceEvent } from '../attendance-context.svelte';
import { eachDayOfMonth, isWeekday } from './attendance-date';
import { computeDayEvents, groupEventsByDay } from './attendance-day-events';
import { formatTimeOfDay } from './attendance-format';

export type PersonalSummaryStats = {
	workedDays: number;
	weekdayCount: number;
	totalMinutes: number;
	averageClockInTime: string;
	topLocationLabel: string;
	topLocationPercent: number;
};

export function computePersonalStats(month: string, events: AttendanceEvent[]): PersonalSummaryStats {
	const byDay = groupEventsByDay(events);
	let workedDays = 0;
	let totalMinutes = 0;
	const clockInMinutes: number[] = [];
	const locationCount = new Map<string, number>();
	for (const [date, dayEvents] of byDay) {
		const day = computeDayEvents(date, dayEvents);
		if (day.clockIn) {
			workedDays += 1;
			totalMinutes += day.workedMinutes;
			clockInMinutes.push(clockInLocalMinutes(day.clockIn));
			const label = day.clockIn.locationName ?? 'Unknown';
			locationCount.set(label, (locationCount.get(label) ?? 0) + 1);
		}
	}
	const weekdayCount = eachDayOfMonth(month).filter(isWeekday).length;
	const averageClockInTime = clockInMinutes.length
		? formatTimeOfDay(clockInMinutes.reduce((a, b) => a + b, 0) / clockInMinutes.length)
		: '--:--';
	let topLocationLabel = '';
	let topLocationCount = 0;
	for (const [label, count] of locationCount) {
		if (count > topLocationCount) {
			topLocationLabel = label;
			topLocationCount = count;
		}
	}
	const topLocationPercent = workedDays ? Math.round((topLocationCount / workedDays) * 100) : 0;
	return { workedDays, weekdayCount, totalMinutes, averageClockInTime, topLocationLabel, topLocationPercent };
}

function clockInLocalMinutes(event: AttendanceEvent): number {
	const [hours, minutes] = event.localTime.split(':').map(Number);
	return (hours || 0) * 60 + (minutes || 0);
}
