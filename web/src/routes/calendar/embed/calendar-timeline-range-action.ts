import type { CalendarViewType } from '@dayflow/core';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import {
	hasTimelinePointerMoved,
	isTimelineView,
	timelineAnchorFromPointerEvent,
	timelineDateFromPointerEvent
} from './calendar-timeline-range-geometry';

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
	isMobileTwoDayWeekView: () => boolean;
	getSelection: () => TimelineRangeSelection | null;
	setSelection: (selection: TimelineRangeSelection | null) => void;
	createSingleEvent: (startDate: Date, anchor: DraftPopoverAnchor) => void;
	createMobileSingleEvent: (startDate: Date) => void;
	createRangeEvent: (firstDate: Date, secondDate: Date, anchor: DraftPopoverAnchor) => void;
};

export function installCalendarTimelineRangeAction(options: TimelineRangeActionOptions): () => void {
	let lastRangeCreationTime = 0;

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
		if (!selection.hasMoved && !hasTimelinePointerMoved(selection, event)) return;
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
		lastRangeCreationTime = Date.now();
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
		const startDate = timelineDateFromPointerEvent(options, event);
		if (!startDate) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		if (Date.now() - lastRangeCreationTime <= 350) return;
		if (options.isMobileTwoDayWeekView()) {
			options.createMobileSingleEvent(startDate);
			return;
		}
		options.createSingleEvent(startDate, timelineAnchorFromPointerEvent(event));
	};

	options.stageElement.addEventListener('pointerdown', handlePointerDown, true);
	document.addEventListener('pointermove', handlePointerMove);
	window.addEventListener('pointerup', handlePointerUp, true);
	window.addEventListener('pointercancel', handlePointerCancel, true);
	options.stageElement.addEventListener('mousedown', handleMouseDown, true);
	options.stageElement.addEventListener('click', handleClick, true);

	return () => {
		options.setSelection(null);
		options.stageElement.removeEventListener('pointerdown', handlePointerDown, true);
		document.removeEventListener('pointermove', handlePointerMove);
		window.removeEventListener('pointerup', handlePointerUp, true);
		window.removeEventListener('pointercancel', handlePointerCancel, true);
		options.stageElement.removeEventListener('mousedown', handleMouseDown, true);
		options.stageElement.removeEventListener('click', handleClick, true);
	};
}

function isTimelineActionEvent(options: TimelineRangeActionOptions, event: MouseEvent | PointerEvent): boolean {
	if (!isTimelineView(options.currentView())) return false;
	if (event.button !== 0 || options.getSelection()) return false;
	return !isIgnoredTimelineTarget(event.target);
}

function isIgnoredTimelineTarget(target: EventTarget | null): boolean {
	if (!(target instanceof Element)) return true;
	return Boolean(
		target.closest(
			'.df-event, .df-all-day-row, .df-week-all-day-shell, .df-day-content-all-day-row, .df-event-detail-panel, .df-dialog-container, [data-range-picker-popup], [data-calendar-picker-dropdown]'
		)
	);
}
