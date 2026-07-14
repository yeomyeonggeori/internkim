import type { AttendanceEvent } from '../attendance-context.svelte';
import { todayDateInTimeZone } from './attendance-date';

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

export type AttendanceWorkSegmentOptions = {
	currentDate?: string;
};

export function buildAttendanceWorkSegments(
	date: string,
	events: AttendanceEvent[],
	options: AttendanceWorkSegmentOptions = {}
): AttendanceWorkSegment[] {
	const sortedEvents = events
		.filter((event) => !event.canceledAt)
		.sort((a, b) => a.occurredAt.localeCompare(b.occurredAt));
	const segments: AttendanceWorkSegment[] = [];
	let openClockIn: AttendanceEvent | undefined;
	for (const event of sortedEvents) {
		if (event.kind === 'clock_in') {
			if (openClockIn) {
				const segment = createAttendanceWorkSegment(openClockIn.localDate, openClockIn, event, 'next_clock_in');
				const splitSegment = splitAttendanceWorkSegmentForDate(date, segment);
				if (splitSegment) segments.push(splitSegment);
			}
			openClockIn = event;
			continue;
		}
		if (!openClockIn) continue;
		const segment = createAttendanceWorkSegment(openClockIn.localDate, openClockIn, event, 'clock_out');
		const splitSegment = splitAttendanceWorkSegmentForDate(date, segment);
		if (splitSegment) segments.push(splitSegment);
		openClockIn = undefined;
	}
	if (openClockIn) {
		const segment = createAttendanceWorkSegment(openClockIn.localDate, openClockIn);
		const splitSegment = splitAttendanceWorkSegmentForDate(date, segment, options.currentDate);
		if (splitSegment) segments.push(splitSegment);
	}
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

function splitAttendanceWorkSegmentForDate(
	date: string,
	segment: AttendanceWorkSegment,
	currentDate?: string
): AttendanceWorkSegment | undefined {
	if (!segment.endEvent) return splitOpenAttendanceWorkSegmentForDate(date, segment, currentDate);
	const startDate = segment.clockIn.localDate;
	const endDate = attendanceSegmentEndDate(segment);
	if (date < startDate || date > endDate) return undefined;
	const startTime = date === startDate ? segment.clockIn.localTime : '00:00:00';
	const endTime = date === endDate ? segment.endEvent.localTime : '24:00:00';
	const workedMinutes = minutesBetweenLocalTimes(startTime, endTime);
	if (workedMinutes <= 0) return undefined;
	return {
		...segment,
		id: `${segment.id}-${date}`,
		date,
		startTime,
		endTime,
		workedMinutes,
		isOpen: false,
	};
}

function attendanceSegmentEndDate(segment: AttendanceWorkSegment): string {
	const endEvent = segment.endEvent;
	if (!endEvent || endEvent.localDate > segment.clockIn.localDate) return endEvent?.localDate ?? segment.date;
	const timeZone = endEvent.timeZoneAtEvent.trim();
	if (!timeZone || timeZone === 'Local') return endEvent.localDate;
	const occurredAt = new Date(endEvent.occurredAt);
	if (Number.isNaN(occurredAt.getTime())) return endEvent.localDate;
	const actualDate = todayDateInTimeZone(timeZone, occurredAt);
	return actualDate > endEvent.localDate ? actualDate : endEvent.localDate;
}

function splitOpenAttendanceWorkSegmentForDate(
	date: string,
	segment: AttendanceWorkSegment,
	currentDate?: string
): AttendanceWorkSegment | undefined {
	const startDate = segment.clockIn.localDate;
	if (date < startDate) return undefined;
	if (!currentDate && date !== startDate) return undefined;
	if (currentDate && date > currentDate) return undefined;
	const startTime = date === startDate ? segment.clockIn.localTime : '00:00:00';
	if (currentDate && date < currentDate) {
		const workedMinutes = minutesBetweenLocalTimes(startTime, '24:00:00');
		if (workedMinutes <= 0) return undefined;
		return {
			...segment,
			id: `${segment.id}-${date}`,
			date,
			startTime,
			endTime: '24:00:00',
			workedMinutes,
			isOpen: false,
		};
	}
	return { ...segment, id: `${segment.id}-${date}`, date, startTime, endTime: undefined, workedMinutes: 0, isOpen: true };
}

function minutesBetweenLocalTimes(startTime: string, endTime: string): number {
	return Math.max(0, localTimeMinutes(endTime) - localTimeMinutes(startTime));
}

function localTimeMinutes(localTime: string): number {
	const [hours = 0, minutes = 0] = localTime.split(':').map(Number);
	return hours * 60 + minutes;
}

function minutesBetween(start: string, end: string): number {
	const startMs = new Date(start).getTime();
	const endMs = new Date(end).getTime();
	if (Number.isNaN(startMs) || Number.isNaN(endMs) || endMs <= startMs) return 0;
	return Math.round((endMs - startMs) / 60000);
}
