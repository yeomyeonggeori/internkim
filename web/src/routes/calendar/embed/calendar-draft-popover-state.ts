// 캘린더 초안 팝오버 상태와 날짜 변환을 관리합니다.
import type { Event as DayFlowEvent } from '@dayflow/core';
import { temporalToDate } from '@dayflow/core';

export type DraftPopoverMode = 'create' | 'edit';

export type DraftPopoverAnchor = {
	clientX: number;
	clientY: number;
	leftClientX?: number;
	topClientY?: number;
	bottomClientY?: number;
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
	width: number;
	arrowTop: number;
	arrowSide: 'left' | 'right';
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
		position: draftPopoverPositionFromAnchor(anchor, stageElement)
	};
}

export function draftPopoverPositionFromAnchor(
	anchor: DraftPopoverAnchor | null,
	stageElement: HTMLElement | null
): DraftPopoverPosition {
	return draftPopoverPosition(anchor, stageElement);
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
	if (!stageRectangle) return { left: 16, top: 16, width: 320, arrowTop: 42, arrowSide: 'left' };
	const stageMargin = 12;
	const arrowOutset = 10;
	const popoverGap = 16;
	const width = Math.min(540, Math.max(320, stageRectangle.width - (stageMargin + arrowOutset) * 2));
	const height = Math.min(520, Math.max(240, stageRectangle.height - stageMargin * 2));
	const fallbackAnchor: DraftPopoverAnchor = {
		clientX: stageRectangle.left + stageRectangle.width / 2,
		clientY: stageRectangle.top + Math.min(220, stageRectangle.height / 3)
	};
	const targetAnchor = anchor ?? fallbackAnchor;
	const titleTop = (targetAnchor.topClientY ?? targetAnchor.clientY) - stageRectangle.top;
	const titleBottom = (targetAnchor.bottomClientY ?? targetAnchor.clientY) - stageRectangle.top;
	const titleCenterY = (titleTop + titleBottom) / 2;
	const preferredRight = targetAnchor.clientX - stageRectangle.left + popoverGap;
	const preferredLeft = (targetAnchor.leftClientX ?? targetAnchor.clientX) - stageRectangle.left - width - popoverGap;
	const rightOverflow = preferredRight + width + stageMargin > stageRectangle.width;
	const leftOverflow = preferredLeft < stageMargin;
	const shouldPlaceRight = !rightOverflow || (leftOverflow && preferredRight <= preferredLeft);
	const arrowSide = shouldPlaceRight ? 'left' : 'right';
	const unclampedLeft = shouldPlaceRight ? preferredRight : preferredLeft;
	const minLeft = stageRectangle.left + stageMargin + (arrowSide === 'left' ? arrowOutset : 0);
	const maxLeft = stageRectangle.right - width - stageMargin - (arrowSide === 'right' ? arrowOutset : 0);
	const left = Math.max(minLeft, Math.min(stageRectangle.left + unclampedLeft, Math.max(minLeft, maxLeft)));
	const sideAlignedTop = titleCenterY - 48;
	const top = Math.max(stageRectangle.top + stageMargin, Math.min(stageRectangle.top + sideAlignedTop, stageRectangle.bottom - height - stageMargin));
	const arrowTop = Math.max(18, Math.min(stageRectangle.top + titleCenterY - top - 8, height - 28));
	return { left, top, width, arrowTop, arrowSide };
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
