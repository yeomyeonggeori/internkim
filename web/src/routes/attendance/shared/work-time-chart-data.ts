import type { AttendanceEvent, AttendanceLocation } from '../attendance-context.svelte';
import { eachDayOfMonth, timeInTimeZone } from './attendance-date';
import { computeDayEvents } from './attendance-day-events';
import type { AttendanceWorkSegment } from './attendance-work-segments';
import { localTimeMinutes } from './day-timeline';
import type { DailyValue, WorkTimeChartLocation } from './work-time-chart-model';

type DailyWorkTimeValuesOptions = {
	currentDate?: string;
	fallbackLocationName: string;
	now?: Date;
};

export function buildDailyWorkTimeValues(
	month: string,
	events: AttendanceEvent[],
	options: DailyWorkTimeValuesOptions
): DailyValue[] {
	if (!month) return [];
	return eachDayOfMonth(month).map((date) => {
		const day = computeDayEvents(date, events, { currentDate: options.currentDate });
		return {
			date,
			minutesByLocation: sumSegmentMinutesByLocation(day.segments, options.fallbackLocationName, options.now),
		};
	});
}

export function buildWorkTimeChartLocations(
	locations: AttendanceLocation[],
	dailyValues: DailyValue[]
): WorkTimeChartLocation[] {
	const chartLocations: WorkTimeChartLocation[] = locations.map((location) => ({
		key: location.name,
		name: location.name,
		color: location.color,
	}));
	const knownKeys = new Set(chartLocations.map((location) => location.key));
	for (const dailyValue of dailyValues) {
		for (const locationKey of Object.keys(dailyValue.minutesByLocation)) {
			if (knownKeys.has(locationKey)) continue;
			knownKeys.add(locationKey);
			const locationByID = locations.find((location) => location.id === locationKey);
			chartLocations.push({
				key: locationKey,
				name: locationByID?.name ?? locationKey,
				color: locationByID?.color,
			});
		}
	}
	return chartLocations;
}

function sumSegmentMinutesByLocation(
	segments: AttendanceWorkSegment[],
	fallbackLocationName: string,
	now?: Date
): Record<string, number> {
	const minutesByLocation: Record<string, number> = {};
	for (const segment of segments) {
		const locationKey = segment.locationName ?? segment.locationID ?? fallbackLocationName;
		const minutes = segmentMinutes(segment, now);
		if (minutes <= 0) continue;
		minutesByLocation[locationKey] = (minutesByLocation[locationKey] ?? 0) + minutes;
	}
	return minutesByLocation;
}

function segmentMinutes(segment: AttendanceWorkSegment, now: Date = new Date()): number {
	if (!segment.isOpen) return segment.workedMinutes;
	const currentTime = timeInTimeZone(segment.clockIn.timeZoneAtEvent, now);
	return Math.max(0, localTimeMinutes(currentTime) - localTimeMinutes(segment.startTime));
}
