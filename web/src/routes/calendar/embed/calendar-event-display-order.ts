import type { Event as DayFlowEvent } from '@dayflow/core';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';

export type CalendarEventDisplayOrderCandidate = {
	eventID: string;
	startDateKey: string;
	endDateKey: string;
	durationDays: number;
	sortTimestamp: number;
	isAllDay: boolean;
	eventIndex: number;
};

export const calendarEventLocalSortMetadataKey = 'localSortAt';

const millisecondsPerDay = 24 * 60 * 60 * 1000;

export function calendarEventDisplayOrderCandidate(
	event: DayFlowEvent,
	eventIndex: number,
	visibleStartDateKey: string,
	visibleEndDateKey: string
): CalendarEventDisplayOrderCandidate | null {
	const eventStartDateKey = calendarDateKey(startOfDay(eventStartDate(event)));
	const eventEndDateKey = calendarDateKey(calendarEventDisplayEndDate(event));
	if (eventEndDateKey < visibleStartDateKey || eventStartDateKey > visibleEndDateKey) return null;
	const startDateKey = eventStartDateKey < visibleStartDateKey ? visibleStartDateKey : eventStartDateKey;
	const endDateKey = eventEndDateKey > visibleEndDateKey ? visibleEndDateKey : eventEndDateKey;
	return {
		eventID: event.id,
		startDateKey,
		endDateKey,
		durationDays: inclusiveDurationDays(startDateKey, endDateKey),
		sortTimestamp: calendarEventSortTimestamp(event),
		isAllDay: event.allDay ?? false,
		eventIndex
	};
}

export function compareCalendarEventDisplayOrderCandidates(
	firstCandidate: CalendarEventDisplayOrderCandidate,
	secondCandidate: CalendarEventDisplayOrderCandidate
): number {
	if (!areOverlappingDateRanges(firstCandidate, secondCandidate)) {
		const startComparison = firstCandidate.startDateKey.localeCompare(secondCandidate.startDateKey);
		if (startComparison !== 0) return startComparison;
	}
	const durationComparison = secondCandidate.durationDays - firstCandidate.durationDays;
	if (durationComparison !== 0) return durationComparison;
	const timestampComparison = secondCandidate.sortTimestamp - firstCandidate.sortTimestamp;
	if (timestampComparison !== 0) return timestampComparison;
	const endComparison = secondCandidate.endDateKey.localeCompare(firstCandidate.endDateKey);
	if (endComparison !== 0) return endComparison;
	const allDayComparison = Number(secondCandidate.isAllDay) - Number(firstCandidate.isAllDay);
	if (allDayComparison !== 0) return allDayComparison;
	const insertionComparison = secondCandidate.eventIndex - firstCandidate.eventIndex;
	if (insertionComparison !== 0) return insertionComparison;
	return firstCandidate.eventID.localeCompare(secondCandidate.eventID);
}

export function calendarDateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

export function calendarEventSortTimestamp(event: DayFlowEvent): number {
	return Math.max(metadataTimestamp(event.meta?.[calendarEventLocalSortMetadataKey]), metadataTimestamp(event.meta?.updatedAt));
}

function areOverlappingDateRanges(
	firstCandidate: CalendarEventDisplayOrderCandidate,
	secondCandidate: CalendarEventDisplayOrderCandidate
): boolean {
	return firstCandidate.startDateKey <= secondCandidate.endDateKey && secondCandidate.startDateKey <= firstCandidate.endDateKey;
}

function inclusiveDurationDays(startDateKey: string, endDateKey: string): number {
	const startDate = dateFromDateKey(startDateKey);
	const endDate = dateFromDateKey(endDateKey);
	return Math.max(1, Math.round((endDate.getTime() - startDate.getTime()) / millisecondsPerDay) + 1);
}

function calendarEventDisplayEndDate(event: DayFlowEvent): Date {
	const startDate = eventStartDate(event);
	const endDate = eventEndDate(event);
	if (event.allDay) return startOfDay(endDate);
	if (!isLocalMidnight(endDate) || endDate.getTime() <= startDate.getTime()) return startOfDay(endDate);
	return addDays(startOfDay(endDate), -1);
}

function dateFromDateKey(dateKey: string): Date {
	const [year = '0', month = '1', day = '1'] = dateKey.split('-');
	return new Date(Number(year), Number(month) - 1, Number(day));
}

function startOfDay(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function addDays(date: Date, days: number): Date {
	const nextDate = new Date(date);
	nextDate.setDate(nextDate.getDate() + days);
	return nextDate;
}

function isLocalMidnight(date: Date): boolean {
	return date.getHours() === 0 && date.getMinutes() === 0 && date.getSeconds() === 0 && date.getMilliseconds() === 0;
}

function metadataTimestamp(value: unknown): number {
	if (typeof value !== 'string') return Number.NEGATIVE_INFINITY;
	const timestamp = Date.parse(value);
	return Number.isFinite(timestamp) ? timestamp : Number.NEGATIVE_INFINITY;
}
