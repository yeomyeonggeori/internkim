import { createEvent, type Event as DayFlowEvent } from '@dayflow/core';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';
import { dateFromDateKey } from './calendar-month-selection';

export type MonthEventWeek = {
	id: string;
	dateKeys: string[];
};

export type MonthEventSegment = {
	id: string;
	eventID: string;
	weekID: string;
	startDateKey: string;
	endDateKey: string;
	lane: number;
	titleText: string;
	endTimeText: string;
	isAllDay: boolean;
};

type MonthEventSegmentCandidate = Omit<MonthEventSegment, 'lane'> & {
	durationDays: number;
	sortTimestamp: number;
	eventIndex: number;
};

type MonthEventLane = {
	endDateKey: string;
};

const millisecondsPerDay = 24 * 60 * 60 * 1000;
const localSortMetadataKey = 'localSortAt';

export function monthEventSegments(events: DayFlowEvent[], weeks: MonthEventWeek[]): MonthEventSegment[] {
	return weeks.flatMap((week) => segmentsForWeek(events, week));
}

export function moveMonthEventToDate(event: DayFlowEvent, targetDateKey: string, movedAt: Date = new Date()): DayFlowEvent {
	const startDate = eventStartDate(event);
	const endDate = eventEndDate(event);
	const targetDate = dateFromDateKey(targetDateKey);
	const sourceDate = localDayStart(startDate);
	const dayDelta = Math.round((targetDate.getTime() - sourceDate.getTime()) / millisecondsPerDay);
	const movedAtISO = movedAt.toISOString();
	return createEvent({
		id: event.id,
		title: event.title,
		description: event.description,
		start: shiftedDate(startDate, dayDelta),
		end: shiftedDate(endDate, dayDelta),
		allDay: event.allDay,
		calendarId: event.calendarId ?? 'internkim',
		meta: {
			...(event.meta ?? {}),
			[localSortMetadataKey]: movedAtISO,
			updatedAt: movedAtISO
		}
	});
}

function segmentsForWeek(events: DayFlowEvent[], week: MonthEventWeek): MonthEventSegment[] {
	const candidates = events
		.flatMap((event, eventIndex) => segmentForEventAndWeek(event, week, eventIndex))
		.sort(compareMonthEventSegmentCandidates);
	const lanes: MonthEventLane[] = [];
	return candidates.map((candidate) => {
		const lane = availableLane(lanes, candidate);
		lanes[lane] = { endDateKey: candidate.endDateKey };
		return {
			id: candidate.id,
			eventID: candidate.eventID,
			weekID: candidate.weekID,
			startDateKey: candidate.startDateKey,
			endDateKey: candidate.endDateKey,
			lane,
			titleText: candidate.titleText,
			endTimeText: candidate.endTimeText,
			isAllDay: candidate.isAllDay
		};
	});
}

function segmentForEventAndWeek(event: DayFlowEvent, week: MonthEventWeek, eventIndex: number): MonthEventSegmentCandidate[] {
	const weekStartDateKey = week.dateKeys[0];
	const weekEndDateKey = week.dateKeys[week.dateKeys.length - 1];
	if (!weekStartDateKey || !weekEndDateKey) return [];
	const eventStartDateKey = dateKey(localDayStart(eventStartDate(event)));
	const eventEndDateKey = dateKey(displayEndDate(event));
	if (eventEndDateKey < weekStartDateKey || eventStartDateKey > weekEndDateKey) return [];
	const startDateKey = eventStartDateKey < weekStartDateKey ? weekStartDateKey : eventStartDateKey;
	const endDateKey = eventEndDateKey > weekEndDateKey ? weekEndDateKey : eventEndDateKey;
	return [
		{
			id: `${event.id}::month-segment::${week.id}`,
			eventID: event.id,
			weekID: week.id,
			startDateKey,
			endDateKey,
			titleText: titleText(event),
			endTimeText: event.allDay ? '' : formatMonthEventTime(eventEndDate(event)),
			isAllDay: event.allDay ?? false,
			durationDays: inclusiveDurationDays(startDateKey, endDateKey),
			sortTimestamp: eventSortTimestamp(event),
			eventIndex
		}
	];
}

function availableLane(lanes: MonthEventLane[], segment: MonthEventSegmentCandidate): number {
	const laneIndex = lanes.findIndex((lane) => lane.endDateKey < segment.startDateKey);
	if (laneIndex >= 0) return laneIndex;
	return lanes.length;
}

function compareMonthEventSegmentCandidates(firstSegment: MonthEventSegmentCandidate, secondSegment: MonthEventSegmentCandidate): number {
	if (!areOverlappingMonthSegments(firstSegment, secondSegment)) {
		const startComparison = firstSegment.startDateKey.localeCompare(secondSegment.startDateKey);
		if (startComparison !== 0) return startComparison;
	}
	const durationComparison = secondSegment.durationDays - firstSegment.durationDays;
	if (durationComparison !== 0) return durationComparison;
	const timestampComparison = secondSegment.sortTimestamp - firstSegment.sortTimestamp;
	if (timestampComparison !== 0) return timestampComparison;
	const endComparison = secondSegment.endDateKey.localeCompare(firstSegment.endDateKey);
	if (endComparison !== 0) return endComparison;
	const allDayComparison = Number(secondSegment.isAllDay) - Number(firstSegment.isAllDay);
	if (allDayComparison !== 0) return allDayComparison;
	const insertionComparison = secondSegment.eventIndex - firstSegment.eventIndex;
	if (insertionComparison !== 0) return insertionComparison;
	return firstSegment.eventID.localeCompare(secondSegment.eventID);
}

function areOverlappingMonthSegments(firstSegment: MonthEventSegmentCandidate, secondSegment: MonthEventSegmentCandidate): boolean {
	return firstSegment.startDateKey <= secondSegment.endDateKey && secondSegment.startDateKey <= firstSegment.endDateKey;
}

function inclusiveDurationDays(startDateKey: string, endDateKey: string): number {
	const startDate = dateFromDateKey(startDateKey);
	const endDate = dateFromDateKey(endDateKey);
	return Math.max(1, Math.round((endDate.getTime() - startDate.getTime()) / millisecondsPerDay) + 1);
}

function eventSortTimestamp(event: DayFlowEvent): number {
	return Math.max(metadataTimestamp(event.meta?.[localSortMetadataKey]), metadataTimestamp(event.meta?.updatedAt));
}

function metadataTimestamp(value: unknown): number {
	if (typeof value !== 'string') return Number.NEGATIVE_INFINITY;
	const timestamp = Date.parse(value);
	return Number.isFinite(timestamp) ? timestamp : Number.NEGATIVE_INFINITY;
}

function titleText(event: DayFlowEvent): string {
	if (event.allDay) return event.title;
	const title = (event.title ?? '').trim();
	const startTime = formatMonthEventTime(eventStartDate(event));
	return title ? `${title} ${startTime}` : startTime;
}

function displayEndDate(event: DayFlowEvent): Date {
	const startDate = eventStartDate(event);
	const endDate = eventEndDate(event);
	if (event.allDay) return localDayStart(endDate);
	if (!isLocalMidnight(endDate) || endDate.getTime() <= startDate.getTime()) return localDayStart(endDate);
	const displayDate = localDayStart(endDate);
	displayDate.setDate(displayDate.getDate() - 1);
	return displayDate;
}

function shiftedDate(date: Date, dayDelta: number): Date {
	const shifted = new Date(date);
	shifted.setDate(shifted.getDate() + dayDelta);
	return shifted;
}

function localDayStart(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function isLocalMidnight(date: Date): boolean {
	return date.getHours() === 0 && date.getMinutes() === 0 && date.getSeconds() === 0 && date.getMilliseconds() === 0;
}

function dateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

function formatMonthEventTime(date: Date): string {
	return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}
