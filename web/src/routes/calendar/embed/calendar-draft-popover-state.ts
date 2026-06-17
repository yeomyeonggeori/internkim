import type { Event as DayFlowEvent } from '@dayflow/core';
import { temporalToDate } from '@dayflow/core';
import {
	draftPopoverPositionFromAnchor,
	type DraftPopoverAnchor,
	type DraftPopoverPosition
} from './calendar-draft-popover-position';

export {
	draftPopoverPositionFromAnchor,
	type DraftPopoverAnchor,
	type DraftPopoverPosition,
	type DraftPopoverSize
} from './calendar-draft-popover-position';

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
	calendarID: string;
	anchor: DraftPopoverAnchor | null;
	position: DraftPopoverPosition;
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
		calendarID: event.calendarId ?? 'internkim',
		anchor,
		position: draftPopoverPositionFromAnchor(anchor, stageElement)
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
			location: popover.location.trim()
		}
	};
}

export function isDraftPopoverValid(popover: DraftPopoverState): boolean {
	if (!popover.title.trim()) return false;
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
