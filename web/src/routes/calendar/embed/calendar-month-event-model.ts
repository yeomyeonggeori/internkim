import { createEvent, type Event as DayFlowEvent } from '@dayflow/core';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';
import {
	calendarEventDisplayOrderCandidate,
	calendarEventLocalSortMetadataKey,
	compareCalendarEventDisplayOrderCandidates
} from './calendar-event-display-order';
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
	titleOnlyText: string;
	startTimeText: string;
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
			[calendarEventLocalSortMetadataKey]: movedAtISO,
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
			titleOnlyText: candidate.titleOnlyText,
			startTimeText: candidate.startTimeText,
			isAllDay: candidate.isAllDay
		};
	});
}

function segmentForEventAndWeek(event: DayFlowEvent, week: MonthEventWeek, eventIndex: number): MonthEventSegmentCandidate[] {
	const weekStartDateKey = week.dateKeys[0];
	const weekEndDateKey = week.dateKeys[week.dateKeys.length - 1];
	if (!weekStartDateKey || !weekEndDateKey) return [];
	const displayOrder = calendarEventDisplayOrderCandidate(event, eventIndex, weekStartDateKey, weekEndDateKey);
	if (!displayOrder) return [];
	return [
		{
			id: `${event.id}::month-segment::${week.id}`,
			weekID: week.id,
			...displayOrder,
			titleText: titleText(event),
			titleOnlyText: titleOnlyText(event),
			startTimeText: event.allDay ? '' : formatMonthEventTime(eventStartDate(event))
		}
	];
}

function availableLane(lanes: MonthEventLane[], segment: MonthEventSegmentCandidate): number {
	const laneIndex = lanes.findIndex((lane) => lane.endDateKey < segment.startDateKey);
	if (laneIndex >= 0) return laneIndex;
	return lanes.length;
}

function compareMonthEventSegmentCandidates(firstSegment: MonthEventSegmentCandidate, secondSegment: MonthEventSegmentCandidate): number {
	return compareCalendarEventDisplayOrderCandidates(firstSegment, secondSegment);
}

function titleText(event: DayFlowEvent): string {
	if (event.allDay) return event.title;
	const title = (event.title ?? '').trim();
	const startTime = formatMonthEventTime(eventStartDate(event));
	return title ? `${title} ${startTime}` : startTime;
}

function titleOnlyText(event: DayFlowEvent): string {
	const title = (event.title ?? '').trim();
	if (title) return title;
	if (event.allDay) return '';
	return formatMonthEventTime(eventStartDate(event));
}

function shiftedDate(date: Date, dayDelta: number): Date {
	const shifted = new Date(date);
	shifted.setDate(shifted.getDate() + dayDelta);
	return shifted;
}

function localDayStart(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function formatMonthEventTime(date: Date): string {
	return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}
