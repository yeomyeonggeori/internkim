import type { AttendanceEvent } from '../attendance-context.svelte';
import { eachDayOfMonth, isWeekday } from './attendance-date';
import { computeDayEvents } from './attendance-day-events';
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
	let workedDays = 0;
	let totalMinutes = 0;
	const clockInMinutes: number[] = [];
	const locationCount = new Map<string, number>();
	for (const date of eachDayOfMonth(month)) {
		const day = computeDayEvents(date, events);
		if (day.segments.length > 0) {
			workedDays += 1;
			totalMinutes += day.workedMinutes;
			clockInMinutes.push(clockInLocalMinutes(day.segments[0]?.startTime));
			for (const segment of day.segments) {
				const label = segment.locationName ?? 'Unknown';
				locationCount.set(label, (locationCount.get(label) ?? 0) + 1);
			}
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
	const segmentCount = [...locationCount.values()].reduce((total, count) => total + count, 0);
	const topLocationPercent = segmentCount ? Math.round((topLocationCount / segmentCount) * 100) : 0;
	return { workedDays, weekdayCount, totalMinutes, averageClockInTime, topLocationLabel, topLocationPercent };
}

function clockInLocalMinutes(localTime: string | undefined): number {
	const [hours, minutes] = (localTime ?? '').split(':').map(Number);
	return (hours || 0) * 60 + (minutes || 0);
}
