import type { DraftEventParams } from './calendar-draft-events';
import { orderedDateKeys, type MonthRangeSelection } from './calendar-month-range-action';
import { dateFromDateKey } from './calendar-month-selection';

const draftEventIDPrefixes = ['quick', 'range', 'month', 'all-day', 'timeline'] as const;
type DraftEventIDPrefix = (typeof draftEventIDPrefixes)[number];

export function quickDraftEventParams(baseDate: Date): DraftEventParams {
	const startDate = new Date(baseDate.getFullYear(), baseDate.getMonth(), baseDate.getDate(), 9, 0, 0, 0);
	const endDate = new Date(baseDate.getFullYear(), baseDate.getMonth(), baseDate.getDate(), 10, 0, 0, 0);
	return {
		id: draftEventID('quick'),
		start: startDate,
		end: endDate,
		allDay: false,
		calendarId: 'internkim'
	};
}

export function monthRangeDraftEventParams(selection: MonthRangeSelection): DraftEventParams | null {
	const [startDateKey, endDateKey] = orderedDateKeys(selection.startDateKey, selection.endDateKey);
	if (startDateKey === endDateKey) return null;
	return {
		id: draftEventID('range'),
		start: dateFromDateKey(startDateKey),
		end: dateFromDateKey(endDateKey),
		allDay: true,
		calendarId: 'internkim'
	};
}

export function monthSingleDayDraftEventParams(dateKey: string): DraftEventParams {
	const startDate = dateFromDateKey(dateKey);
	startDate.setHours(9, 0, 0, 0);
	const endDate = new Date(startDate);
	endDate.setHours(10, 0, 0, 0);
	return {
		id: draftEventID('month'),
		start: startDate,
		end: endDate,
		allDay: false,
		calendarId: 'internkim'
	};
}

export function allDaySingleDraftEventParams(dateKey: string): DraftEventParams {
	const startDate = dateFromDateKey(dateKey);
	return {
		id: draftEventID('all-day'),
		start: startDate,
		end: new Date(startDate),
		allDay: true,
		calendarId: 'internkim'
	};
}

export function timelineSingleDraftEventParams(startDate: Date): DraftEventParams {
	const normalizedStartDate = normalizedTimelineDate(startDate);
	const endDate = new Date(normalizedStartDate);
	endDate.setHours(normalizedStartDate.getHours() + 1, normalizedStartDate.getMinutes(), 0, 0);
	return {
		id: draftEventID('timeline'),
		start: normalizedStartDate,
		end: endDate,
		allDay: false,
		calendarId: 'internkim'
	};
}

export function timelineRangeDraftEventParams(firstDate: Date, secondDate: Date): DraftEventParams {
	const [startDate, endDate] = orderedTimelineRangeDates(firstDate, secondDate);
	return {
		id: draftEventID('timeline'),
		start: startDate,
		end: endDate,
		allDay: false,
		calendarId: 'internkim'
	};
}

export function orderedTimelineRangeDates(firstDate: Date, secondDate: Date): [Date, Date] {
	const normalizedFirstDate = normalizedTimelineDate(firstDate);
	const normalizedSecondDate = normalizedTimelineDate(secondDate);
	const startDate =
		normalizedFirstDate.getTime() <= normalizedSecondDate.getTime() ? normalizedFirstDate : normalizedSecondDate;
	const endDate =
		normalizedFirstDate.getTime() <= normalizedSecondDate.getTime() ? normalizedSecondDate : normalizedFirstDate;
	if (endDate.getTime() - startDate.getTime() < 30 * 60 * 1000) {
		endDate.setTime(startDate.getTime() + 30 * 60 * 1000);
	}
	return [startDate, endDate];
}

function normalizedTimelineDate(date: Date): Date {
	const normalizedDate = new Date(date);
	normalizedDate.setSeconds(0, 0);
	return normalizedDate;
}

export function isDraftEventID(eventID: string): boolean {
	return draftEventIDPrefixes.some((prefix) => eventID.startsWith(`${prefix}-`));
}

function draftEventID(prefix: DraftEventIDPrefix): string {
	return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}
