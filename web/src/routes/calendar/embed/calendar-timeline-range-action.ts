import type { CalendarViewType } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';

export type TimelineRangeSelection = {
	pointerID: number;
	startClientX: number;
	startClientY: number;
	startDate: Date;
	currentDate: Date;
	currentAnchor: DraftPopoverAnchor;
	hasMoved: boolean;
};

export type TimelineRangeActionOptions = {
	stageElement: HTMLElement;
	currentView: () => CalendarViewType;
	currentDate: () => Date;
	getSelection: () => TimelineRangeSelection | null;
	setSelection: (selection: TimelineRangeSelection | null) => void;
	createSingleEvent: (startDate: Date, anchor: DraftPopoverAnchor) => void;
	createRangeEvent: (firstDate: Date, secondDate: Date, anchor: DraftPopoverAnchor) => void;
};

const timelineRangeMoveThresholdPx = 8;
const timelineRangeMinuteStep = 15;
const timelineHourHeightPx = 72;

export function installCalendarTimelineRangeAction(options: TimelineRangeActionOptions): () => void {
	const handlePointerDown = (event: PointerEvent) => {
		if (!isTimelineActionEvent(options, event)) return;
		const startDate = timelineDateFromPointerEvent(options, event);
		if (!startDate) return;
		event.preventDefault();
		options.setSelection({
			pointerID: event.pointerId,
			startClientX: event.clientX,
			startClientY: event.clientY,
			startDate,
			currentDate: startDate,
			currentAnchor: timelineAnchorFromPointerEvent(event),
			hasMoved: false
		});
	};

	const handlePointerMove = (event: PointerEvent) => {
		const selection = options.getSelection();
		if (!selection || selection.pointerID !== event.pointerId) return;
		if (!selection.hasMoved && !hasPointerMoved(selection, event)) return;
		const currentDate = timelineDateFromPointerEvent(options, event, false);
		if (!currentDate) return;
		event.preventDefault();
		options.setSelection({ ...selection, currentDate, currentAnchor: timelineAnchorFromPointerEvent(event), hasMoved: true });
	};

	const handlePointerUp = (event: PointerEvent) => {
		const selection = options.getSelection();
		if (!selection || selection.pointerID !== event.pointerId) return;
		options.setSelection(null);
		if (!selection.hasMoved) return;
		const endDate = timelineDateFromPointerEvent(options, event, false) ?? selection.currentDate;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		options.createRangeEvent(selection.startDate, endDate, selection.currentAnchor);
	};

	const handlePointerCancel = (event: PointerEvent) => {
		const selection = options.getSelection();
		if (!selection || selection.pointerID !== event.pointerId) return;
		options.setSelection(null);
	};

	const handleMouseDown = (event: MouseEvent) => {
		if (!isTimelineView(options.currentView())) return;
		if (event.button !== 0 || isIgnoredTimelineTarget(event.target)) return;
		if (!timelineDateFromPointerEvent(options, event)) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
	};

	const handleClick = (event: MouseEvent) => {
		if (!isTimelineActionEvent(options, event)) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
	};

	const handleDoubleClick = (event: MouseEvent) => {
		if (!isTimelineActionEvent(options, event)) return;
		const startDate = timelineDateFromPointerEvent(options, event);
		if (!startDate) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		options.createSingleEvent(startDate, timelineAnchorFromPointerEvent(event));
	};

	options.stageElement.addEventListener('pointerdown', handlePointerDown, true);
	document.addEventListener('pointermove', handlePointerMove);
	window.addEventListener('pointerup', handlePointerUp, true);
	window.addEventListener('pointercancel', handlePointerCancel, true);
	options.stageElement.addEventListener('mousedown', handleMouseDown, true);
	options.stageElement.addEventListener('click', handleClick, true);
	options.stageElement.addEventListener('dblclick', handleDoubleClick, true);

	return () => {
		options.setSelection(null);
		options.stageElement.removeEventListener('pointerdown', handlePointerDown, true);
		document.removeEventListener('pointermove', handlePointerMove);
		window.removeEventListener('pointerup', handlePointerUp, true);
		window.removeEventListener('pointercancel', handlePointerCancel, true);
		options.stageElement.removeEventListener('mousedown', handleMouseDown, true);
		options.stageElement.removeEventListener('click', handleClick, true);
		options.stageElement.removeEventListener('dblclick', handleDoubleClick, true);
	};
}

function timelineDateFromPointerEvent(
	options: Pick<TimelineRangeActionOptions, 'currentDate' | 'currentView' | 'stageElement'>,
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

function isTimelineActionEvent(options: TimelineRangeActionOptions, event: MouseEvent | PointerEvent): boolean {
	if (!isTimelineView(options.currentView())) return false;
	if (event.button !== 0 || options.getSelection()) return false;
	return !isIgnoredTimelineTarget(event.target);
}

function isTimelineView(view: CalendarViewType): boolean {
	return view === ViewType.DAY || view === ViewType.WEEK;
}

function isIgnoredTimelineTarget(target: EventTarget | null): boolean {
	if (!(target instanceof Element)) return true;
	return Boolean(
		target.closest(
			'.df-event, .df-all-day-row, .df-week-all-day-shell, .df-day-content-all-day-row, .df-event-detail-panel, .df-dialog-container, [data-range-picker-popup], [data-calendar-picker-dropdown]'
		)
	);
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

function timelineAnchorFromPointerEvent(event: MouseEvent | PointerEvent): DraftPopoverAnchor {
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

function startOfWeek(date: Date): Date {
	const startDate = new Date(date);
	startDate.setDate(startDate.getDate() - startDate.getDay());
	startDate.setHours(0, 0, 0, 0);
	return startDate;
}

function hasPointerMoved(selection: TimelineRangeSelection, event: PointerEvent): boolean {
	return Math.hypot(event.clientX - selection.startClientX, event.clientY - selection.startClientY) > timelineRangeMoveThresholdPx;
}
