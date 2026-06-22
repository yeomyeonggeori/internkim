import type { CalendarViewType } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';

type TimelineRangeGeometryOptions = {
	stageElement: HTMLElement;
	currentView: () => CalendarViewType;
	currentDate: () => Date;
};

type TimelineRangePointerStart = {
	startClientX: number;
	startClientY: number;
};

const timelineRangeMoveThresholdPx = 8;
const timelineRangeMinuteStep = 15;
const timelineHourHeightPx = 72;

export function timelineDateFromPointerEvent(
	options: TimelineRangeGeometryOptions,
	event: MouseEvent | PointerEvent,
	requireGridTarget = true
): Date | null {
	if (!isTimelineView(options.currentView())) return null;
	if (requireGridTarget && !isTimelineGridTarget(event.target)) return null;
	const timelineTimeSource = timelineTimeSourceElement(options.stageElement, event.target);
	const firstGridRow = timelineFirstGridRow(options.stageElement);
	if (!timelineTimeSource || !firstGridRow) return null;
	const rawHour = timelinePointerHour(timelineTimeSource, firstGridRow, event.clientY);
	const columnIndex = timelineColumnIndex(firstGridRow, event.clientX, options.currentView());
	const date = timelineDateForColumn(options.currentDate(), options.currentView(), columnIndex);
	return timelineDateWithHour(date, rawHour, timelineRangeMinuteStep);
}

export function timelineAnchorFromPointerEvent(event: MouseEvent | PointerEvent): DraftPopoverAnchor {
	const cellElement =
		event.target instanceof Element
			? event.target.closest<HTMLElement>('.df-week-time-grid-cell, .df-day-content-grid-column')
			: null;
	if (cellElement) {
		const rectangle = cellElement.getBoundingClientRect();
		return {
			clientX: rectangle.right,
			clientY: event.clientY,
			leftClientX: rectangle.left,
			topClientY: event.clientY - 8,
			bottomClientY: event.clientY + 8
		};
	}
	return {
		clientX: event.clientX,
		clientY: event.clientY,
		leftClientX: event.clientX - 8,
		topClientY: event.clientY - 8,
		bottomClientY: event.clientY + 8
	};
}

export function isTimelineView(view: CalendarViewType): boolean {
	return view === ViewType.DAY || view === ViewType.WEEK;
}

export function hasTimelinePointerMoved(selection: TimelineRangePointerStart, event: PointerEvent): boolean {
	return Math.hypot(event.clientX - selection.startClientX, event.clientY - selection.startClientY) > timelineRangeMoveThresholdPx;
}

function isTimelineGridTarget(target: EventTarget | null): boolean {
	if (!(target instanceof Element)) return false;
	return Boolean(
		target.closest(
			'.df-time-grid-cell, .df-week-time-grid-cell, .df-day-content-grid-column, .df-day-content-grid, .df-week-time-grid-grid'
		)
	);
}

function timelineTimeSourceElement(stageElement: HTMLElement, target: EventTarget | null): HTMLElement | null {
	if (target instanceof Element) {
		const closestElement = target.closest<HTMLElement>('.df-week-time-grid-scroller, .df-day-content-grid-rows, .df-day-content-grid');
		if (closestElement) return closestElement;
	}
	const element = stageElement.querySelector('.df-week-time-grid-scroller, .df-day-content-grid-rows, .df-day-content-grid, .df-calendar-content');
	if (!(element instanceof HTMLElement)) return null;
	return element;
}

function timelineFirstGridRow(stageElement: HTMLElement): HTMLElement | null {
	const element = stageElement.querySelector('.df-time-grid-row');
	if (!(element instanceof HTMLElement)) return null;
	return element;
}

function timelinePointerHour(timelineTimeSource: HTMLElement, firstGridRow: HTMLElement, clientY: number): number {
	if (timelineTimeSource.classList.contains('df-day-content-grid-rows')) {
		return timelinePointerHourFromDayRows(timelineTimeSource, clientY);
	}
	const contentRectangle = timelineTimeSource.getBoundingClientRect();
	const firstGridRectangle = firstGridRow.getBoundingClientRect();
	const gridOffset = firstGridRectangle.top - contentRectangle.top + timelineTimeSource.scrollTop;
	const rowHeight = firstGridRectangle.height > 0 ? firstGridRectangle.height : timelineHourHeightPx;
	const relativeY = clientY - contentRectangle.top + timelineTimeSource.scrollTop - gridOffset;
	return Math.max(0, Math.min(24, relativeY / rowHeight));
}

function timelinePointerHourFromDayRows(dayRowsElement: HTMLElement, clientY: number): number {
	const contentRectangle = dayRowsElement.getBoundingClientRect();
	const contentHeight = Math.max(dayRowsElement.scrollHeight, contentRectangle.height, timelineHourHeightPx * 24);
	const hourHeight = contentHeight / 24;
	const relativeY = clientY - contentRectangle.top + dayRowsElement.scrollTop;
	return Math.max(0, Math.min(24, relativeY / hourHeight));
}

function timelineColumnIndex(firstGridRow: HTMLElement, clientX: number, view: CalendarViewType): number {
	const displayDays = view === ViewType.WEEK ? 7 : 1;
	const gridRectangle = firstGridRow.getBoundingClientRect();
	if (gridRectangle.width <= 0) return 0;
	const rawColumnIndex = Math.floor(((clientX - gridRectangle.left) / gridRectangle.width) * displayDays);
	return Math.max(0, Math.min(displayDays - 1, rawColumnIndex));
}

function timelineDateForColumn(currentDate: Date, view: CalendarViewType, columnIndex: number): Date {
	const date = view === ViewType.WEEK ? startOfWeek(currentDate) : new Date(currentDate);
	date.setDate(date.getDate() + columnIndex);
	date.setHours(0, 0, 0, 0);
	return date;
}

function timelineDateWithHour(date: Date, rawHour: number, minuteStep: number): Date {
	const minutes = Math.round((rawHour * 60) / minuteStep) * minuteStep;
	const nextDate = new Date(date);
	nextDate.setHours(0, minutes, 0, 0);
	return nextDate;
}

function startOfWeek(date: Date): Date {
	const startDate = new Date(date);
	startDate.setDate(startDate.getDate() - startDate.getDay());
	startDate.setHours(0, 0, 0, 0);
	return startDate;
}
