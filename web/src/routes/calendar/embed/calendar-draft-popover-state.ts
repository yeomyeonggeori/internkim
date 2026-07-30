import type { Event as DayFlowEvent } from '@dayflow/core';
import { temporalToDate } from '@dayflow/core';
import { calendarDateTimeRangeChangesForStart } from './calendar-date-time-range';
import type { DraftPopoverAnchor } from './calendar-draft-popover-anchor-types';
import {
	calendarParticipantsEqual,
	calendarParticipantsFromUnknown,
	type CalendarParticipant
} from './calendar-participants';

export type { DraftPopoverAnchor } from './calendar-draft-popover-anchor-types';

export type DraftPopoverMode = 'create' | 'edit';

export type DraftPopoverState = {
	mode: DraftPopoverMode;
	eventID: string;
	title: string;
	dateKey: string;
	endDateKey: string;
	startTime: string;
	endTime: string;
	allDay: boolean;
	location: string;
	description: string;
	participants: CalendarParticipant[];
	calendarID: string;
	anchor: DraftPopoverAnchor | null;
};

export type DraftPopoverEventChanges = {
	title: string;
	description: string;
	start: Date;
	end: Date;
	allDay: boolean;
	calendarId: string;
	meta: Record<string, unknown>;
};

type DraftPopoverDateTimeState = Pick<DraftPopoverState, 'dateKey' | 'endDateKey' | 'startTime' | 'endTime' | 'allDay'>;

const defaultTimedStartTime = '09:00';
const defaultTimedEndTime = '10:00';

export function draftPopoverStateFromEvent(
	event: DayFlowEvent,
	mode: DraftPopoverMode,
	anchor: DraftPopoverAnchor | null,
	stageElement: HTMLElement | null
): DraftPopoverState {
	const startDate = dateFromEventValue(event.start);
	const endDate = dateFromEventValue(event.end);
	return {
		mode,
		eventID: event.id,
		title: event.title ?? '',
		dateKey: dateKey(startDate),
		endDateKey: dateKey(endDate),
		startTime: timeValue(startDate),
		endTime: timeValue(endDate),
		allDay: event.allDay ?? false,
		location: typeof event.meta?.location === 'string' ? event.meta.location : '',
		description: event.description ?? '',
		participants: calendarParticipantsFromUnknown(event.meta?.participants),
		calendarID: event.calendarId ?? 'internkim',
		anchor
	};
}

export function draftPopoverAllDayChanges(popover: DraftPopoverState, allDay: boolean): Partial<DraftPopoverState> {
	if (allDay || !shouldUseDefaultTimedValues(popover)) return { allDay };
	return {
		allDay,
		startTime: defaultTimedStartTime,
		endTime: defaultTimedEndTime
	};
}

export function draftPopoverStartDateTimeChanges(
	popover: DraftPopoverDateTimeState,
	nextDateKey: string,
	nextStartTime: string
): Partial<DraftPopoverState> {
	const changes = calendarDateTimeRangeChangesForStart(
		{
			startDateKey: popover.dateKey,
			endDateKey: popover.endDateKey,
			startTime: popover.startTime,
			endTime: popover.endTime,
			allDay: popover.allDay
		},
		{
			startDateKey: nextDateKey,
			startTime: nextStartTime
		}
	);
	return {
		dateKey: changes.startDateKey,
		startTime: changes.startTime,
		endDateKey: changes.endDateKey,
		endTime: changes.endTime
	};
}

export function draftPopoverChanges(popover: DraftPopoverState): DraftPopoverEventChanges {
	const start = draftPopoverStartDate(popover);
	const end = draftPopoverEndDate(popover);
	return {
		title: popover.title.trim(),
		description: popover.description.trim(),
		start,
		end,
		allDay: popover.allDay,
		calendarId: popover.calendarID,
		meta: {
			location: popover.location.trim(),
			participants: popover.participants
		}
	};
}

export function hasDraftPopoverEventChanges(popover: DraftPopoverState, event: DayFlowEvent): boolean {
	const changes = draftPopoverChanges(popover);
	return (
		normalizedText(event.title) !== changes.title ||
		normalizedText(event.description) !== changes.description ||
		dateFromEventValue(event.start).getTime() !== changes.start.getTime() ||
		dateFromEventValue(event.end).getTime() !== changes.end.getTime() ||
		(event.allDay ?? false) !== changes.allDay ||
		(event.calendarId ?? 'internkim') !== changes.calendarId ||
		eventLocation(event) !== normalizedText(changes.meta.location) ||
		!calendarParticipantsEqual(calendarParticipantsFromUnknown(event.meta?.participants), popover.participants)
	);
}

export function isDraftPopoverValid(popover: DraftPopoverState): boolean {
	const startTime = draftPopoverStartDate(popover).getTime();
	const endTime = draftPopoverEndDate(popover).getTime();
	return popover.allDay ? endTime >= startTime : endTime > startTime;
}

export function draftPopoverStartDate(popover: DraftPopoverState): Date {
	return draftPopoverDateTime(popover.dateKey, popover.allDay ? '00:00' : popover.startTime);
}

export function draftPopoverEndDate(popover: DraftPopoverState): Date {
	if (popover.allDay) return draftPopoverDateTime(popover.endDateKey, '00:00');
	return draftPopoverDateTime(popover.endDateKey, popover.endTime);
}

export function dateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

function dateFromEventValue(value: DayFlowEvent['start']): Date {
	return temporalToDate(value);
}

function eventLocation(event: DayFlowEvent): string {
	return normalizedText(event.meta?.location);
}

function normalizedText(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}

function draftPopoverDateTime(selectedDateKey: string, time: string): Date {
	const [year = '1970', month = '1', day = '1'] = selectedDateKey.split('-');
	const [hour = '0', minute = '0'] = time.split(':');
	return new Date(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute), 0, 0);
}

function timeValue(date: Date): string {
	return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

function shouldUseDefaultTimedValues(popover: DraftPopoverState): boolean {
	return popover.allDay && popover.startTime === '00:00' && popover.endTime === '00:00';
}
