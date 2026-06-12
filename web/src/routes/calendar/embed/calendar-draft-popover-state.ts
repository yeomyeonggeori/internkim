// 캘린더 초안 팝오버 상태와 날짜 변환을 관리합니다.
import type { Event as DayFlowEvent } from '@dayflow/core';
import { temporalToDate } from '@dayflow/core';

export type DraftPopoverMode = 'create' | 'edit';

export type DraftPopoverAnchor = {
	left: number;
	top: number;
	width: number;
	height: number;
};

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
	position: DraftPopoverPosition;
};

export type DraftPopoverPosition = {
	left: number;
	top: number;
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
		position: draftPopoverPosition(anchor, stageElement)
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

function draftPopoverPosition(anchor: DraftPopoverAnchor | null, stageElement: HTMLElement | null): DraftPopoverPosition {
	const stageRectangle = stageElement?.getBoundingClientRect();
	const fallbackLeft = stageRectangle ? stageRectangle.left + Math.min(420, stageRectangle.width / 2) : 320;
	const fallbackTop = stageRectangle ? stageRectangle.top + 72 : 88;
	if (!anchor) return { left: fallbackLeft, top: fallbackTop };
	const desiredLeft = anchor.left + anchor.width + 12;
	const maxLeft = window.innerWidth - 380;
	const left = Math.max(16, Math.min(desiredLeft, maxLeft));
	const top = Math.max(16, Math.min(anchor.top, window.innerHeight - 560));
	return { left, top };
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
