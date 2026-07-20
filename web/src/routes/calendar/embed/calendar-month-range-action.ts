import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import {
	hasMonthRangePointerMoved,
	monthDateAnchorFromCell,
	monthDateAnchorFromPoint,
	monthDateCellFromEventTarget,
	monthDateCellFromPoint
} from './calendar-month-range-hit-test';
export {
	monthRangePreviewSegmentsFromSelection,
	orderedDateKeys,
	type MonthRangePreviewSegment
} from './calendar-month-range-preview-segments';

export type MonthRangeSelection = {
	pointerID: number;
	startDateKey: string;
	endDateKey: string;
	startClientX: number;
	startClientY: number;
	hasMoved: boolean;
};

export type MonthRangeActionOptions = {
	stageElement: HTMLElement;
	currentView: () => string;
	getSelection: () => MonthRangeSelection | null;
	setSelection: (selection: MonthRangeSelection | null) => void;
	clearPreview: () => void;
	selectDate: (dateKey: string) => void;
	createSingleDayEvent: (dateKey: string, anchor: DraftPopoverAnchor) => void;
	createRangeEvent: (selection: MonthRangeSelection, anchor: DraftPopoverAnchor) => void;
};

export function installCalendarMonthRangeAction(options: MonthRangeActionOptions): () => void {
	let lastRangeCreationTime = 0;

	const handlePointerDown = (event: PointerEvent) => {
		if (event.button !== 0 || options.getSelection()) return;
		const dateCell = monthDateCellFromEventTarget(options, event.target);
		if (!dateCell) return;
		options.setSelection({
			pointerID: event.pointerId,
			startDateKey: dateCell.dateKey,
			endDateKey: dateCell.dateKey,
			startClientX: event.clientX,
			startClientY: event.clientY,
			hasMoved: false
		});
	};

	const handlePointerMove = (event: PointerEvent) => {
		const selection = options.getSelection();
		if (!selection || selection.pointerID !== event.pointerId) return;
		const hasMoved = selection.hasMoved || hasMonthRangePointerMoved(selection, event);
		const dateCell = monthDateCellFromPoint(event.clientX, event.clientY);
		if (!dateCell && selection.hasMoved === hasMoved) return;
		if (hasMoved) {
			event.preventDefault();
		}
		options.setSelection({
			...selection,
			endDateKey: dateCell?.dateKey ?? selection.endDateKey,
			hasMoved
		});
	};

	const handlePointerUp = (event: PointerEvent) => {
		const selection = options.getSelection();
		if (!selection || selection.pointerID !== event.pointerId) return;
		options.setSelection(null);
		options.clearPreview();
		if (!selection.hasMoved) {
			options.selectDate(selection.startDateKey);
			return;
		}
		event.preventDefault();
		event.stopPropagation();
		lastRangeCreationTime = Date.now();
		options.createRangeEvent(selection, monthDateAnchorFromPoint(event.clientX, event.clientY));
	};

	const handlePointerCancel = (event: PointerEvent) => {
		const selection = options.getSelection();
		if (!selection || selection.pointerID !== event.pointerId) return;
		options.setSelection(null);
		options.clearPreview();
	};

	const handleClick = (event: MouseEvent) => {
		const dateCell = monthDateCellFromEventTarget(options, event.target);
		if (!dateCell) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		if (Date.now() - lastRangeCreationTime <= 350) return;
		options.createSingleDayEvent(dateCell.dateKey, monthDateAnchorFromCell(dateCell.element));
	};

	options.stageElement.addEventListener('pointerdown', handlePointerDown);
	document.addEventListener('pointermove', handlePointerMove);
	document.addEventListener('pointerup', handlePointerUp);
	document.addEventListener('pointercancel', handlePointerCancel);
	options.stageElement.addEventListener('click', handleClick, true);

	return () => {
		options.stageElement.removeEventListener('pointerdown', handlePointerDown);
		document.removeEventListener('pointermove', handlePointerMove);
		document.removeEventListener('pointerup', handlePointerUp);
		document.removeEventListener('pointercancel', handlePointerCancel);
		options.stageElement.removeEventListener('click', handleClick, true);
	};
}
