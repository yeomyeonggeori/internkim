import type { CalendarParticipant } from '../embed/calendar-participants';
import {
	addCalendarGridDays,
	calendarGridMinutesFromMidnight,
	isSameCalendarGridDay,
	startOfCalendarGridDay,
	type CalendarGridWeek
} from './calendar-grid-dates';

export type CalendarGridEvent = {
	id: string;
	title: string;
	start: Date;
	end: Date;
	isAllDay: boolean;
	color: string;
	participants: CalendarParticipant[];
	readOnly?: boolean;
	displayPriority?: number;
};

export type CalendarGridSpan = {
	event: CalendarGridEvent;
	startColumn: number;
	columnCount: number;
	lane: number;
	continuesBefore: boolean;
	continuesAfter: boolean;
};

export type CalendarGridDayEntry = {
	event: CalendarGridEvent;
	dayIndex: number;
};

export type CalendarGridWeekLayout = {
	spans: CalendarGridSpan[];
	timedEntries: CalendarGridDayEntry[];
	laneCount: number;
};

export type CalendarGridTimedBlock = {
	event: CalendarGridEvent;
	startMinutes: number;
	endMinutes: number;
	column: number;
	columnCount: number;
};

const minimumTimedBlockMinutes = 15;

export function calendarGridAllDaySpans(days: Date[], events: CalendarGridEvent[]): CalendarGridSpan[] {
	if (days.length === 0) return [];
	const rangeStart = startOfCalendarGridDay(days[0]);
	const rangeEnd = addCalendarGridDays(rangeStart, days.length);
	const laneEnds: number[] = [];
	return events
		.filter((event) => event.isAllDay && event.end > rangeStart && event.start < rangeEnd)
		.sort(compareSpanEvents)
		.map((event) => {
			const startColumn = Math.max(0, dayDifference(rangeStart, event.start));
			const endColumn = Math.min(days.length - 1, dayDifference(rangeStart, lastCoveredDay(event)));
			const columnCount = Math.max(1, endColumn - startColumn + 1);
			const lane = firstFreeLane(laneEnds, startColumn);
			laneEnds[lane] = startColumn + columnCount;
			return {
				event,
				startColumn,
				columnCount,
				lane,
				continuesBefore: event.start < rangeStart,
				continuesAfter: lastCoveredDay(event) >= rangeEnd
			};
		});
}

export function calendarGridWeekLayout(week: CalendarGridWeek, events: CalendarGridEvent[]): CalendarGridWeekLayout {
	const weekStart = week.days[0];
	const weekEnd = addCalendarGridDays(weekStart, 7);
	const visibleEvents = events.filter((event) => event.end > weekStart && event.start < weekEnd);
	const spanEvents = visibleEvents.filter(isSpanEvent).sort(compareSpanEvents);
	const laneEnds: number[] = [];
	const spans = spanEvents.map((event) => {
		const startColumn = Math.max(0, dayDifference(weekStart, event.start));
		const endColumn = Math.min(6, dayDifference(weekStart, lastCoveredDay(event)));
		const columnCount = Math.max(1, endColumn - startColumn + 1);
		const lane = firstFreeLane(laneEnds, startColumn);
		laneEnds[lane] = startColumn + columnCount;
		return {
			event,
			startColumn,
			columnCount,
			lane,
			continuesBefore: event.start < weekStart,
			continuesAfter: lastCoveredDay(event) >= weekEnd
		};
	});
	const timedEntries = visibleEvents
		.filter((event) => !isSpanEvent(event))
		.map((event) => ({ event, dayIndex: dayDifference(weekStart, event.start) }))
		.filter((entry) => entry.dayIndex >= 0 && entry.dayIndex <= 6)
		.sort(
			(first, second) =>
				first.dayIndex - second.dayIndex ||
				compareEventPriority(first.event, second.event) ||
				first.event.start.getTime() - second.event.start.getTime()
		);
	return { spans, timedEntries, laneCount: laneEnds.length };
}

export function calendarGridTimedBlocks(day: Date, events: CalendarGridEvent[]): CalendarGridTimedBlock[] {
	const dayStart = startOfCalendarGridDay(day);
	const dayEnd = addCalendarGridDays(dayStart, 1);
	const timedEvents = events
		.filter((event) => !event.isAllDay && event.end > dayStart && event.start < dayEnd)
		.sort((first, second) => first.start.getTime() - second.start.getTime() || second.end.getTime() - first.end.getTime());
	const blocks = timedEvents.map((event) => ({
		event,
		startMinutes: event.start <= dayStart ? 0 : calendarGridMinutesFromMidnight(event.start),
		endMinutes: event.end >= dayEnd ? 24 * 60 : calendarGridMinutesFromMidnight(event.end),
		column: 0,
		columnCount: 1
	}));
	return assignOverlapColumns(blocks);
}

function assignOverlapColumns(blocks: CalendarGridTimedBlock[]): CalendarGridTimedBlock[] {
	let cluster: CalendarGridTimedBlock[] = [];
	let clusterEnd = -1;
	const laidOutBlocks: CalendarGridTimedBlock[] = [];

	const closeCluster = () => {
		const columnCount = Math.max(1, ...cluster.map((block) => block.column + 1));
		for (const block of cluster) laidOutBlocks.push({ ...block, columnCount });
		cluster = [];
		clusterEnd = -1;
	};

	for (const block of blocks) {
		const blockEnd = Math.max(block.endMinutes, block.startMinutes + minimumTimedBlockMinutes);
		if (cluster.length > 0 && block.startMinutes >= clusterEnd) closeCluster();
		const takenColumns = new Set(
			cluster
				.filter((placed) => Math.max(placed.endMinutes, placed.startMinutes + minimumTimedBlockMinutes) > block.startMinutes)
				.map((placed) => placed.column)
		);
		let column = 0;
		while (takenColumns.has(column)) column += 1;
		cluster.push({ ...block, column });
		clusterEnd = Math.max(clusterEnd, blockEnd);
	}
	if (cluster.length > 0) closeCluster();
	return laidOutBlocks;
}

function isSpanEvent(event: CalendarGridEvent): boolean {
	return event.isAllDay || !isSameCalendarGridDay(event.start, lastCoveredDay(event));
}

function lastCoveredDay(event: CalendarGridEvent): Date {
	const inclusiveEnd = new Date(event.end.getTime() - 1);
	return inclusiveEnd < event.start ? startOfCalendarGridDay(event.start) : startOfCalendarGridDay(inclusiveEnd);
}

function compareSpanEvents(first: CalendarGridEvent, second: CalendarGridEvent): number {
	return (
		compareEventPriority(first, second) ||
		first.start.getTime() - second.start.getTime() ||
		second.end.getTime() - first.end.getTime() ||
		first.title.localeCompare(second.title)
	);
}

function compareEventPriority(first: CalendarGridEvent, second: CalendarGridEvent): number {
	return (first.displayPriority ?? 1) - (second.displayPriority ?? 1);
}

function firstFreeLane(laneEnds: number[], startColumn: number): number {
	for (let lane = 0; lane < laneEnds.length; lane += 1) {
		if (laneEnds[lane] <= startColumn) return lane;
	}
	return laneEnds.length;
}

function dayDifference(from: Date, to: Date): number {
	const fromStart = startOfCalendarGridDay(from).getTime();
	const toStart = startOfCalendarGridDay(to).getTime();
	return Math.round((toStart - fromStart) / 86400000);
}
