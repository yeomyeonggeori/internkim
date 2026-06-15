import { ViewType } from '@dayflow/svelte';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';

export type MonthRangeSelection = {
	pointerID: number;
	startDateKey: string;
	endDateKey: string;
	startClientX: number;
	startClientY: number;
	hasMoved: boolean;
};

export type MonthRangePreviewSegment = {
	id: string;
	left: number;
	top: number;
	width: number;
	height: number;
};

type MonthDateCell = {
	element: HTMLElement;
	dateKey: string;
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
		const hasMoved = selection.hasMoved || hasPointerMoved(selection, event);
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

	const handleClick = (event: MouseEvent) => {
		if (Date.now() - lastRangeCreationTime > 350) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
	};

	const handleDoubleClick = (event: MouseEvent) => {
		const dateCell = monthDateCellFromEventTarget(options, event.target);
		if (!dateCell) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		options.createSingleDayEvent(dateCell.dateKey, monthDateAnchorFromCell(dateCell.element));
	};

	options.stageElement.addEventListener('pointerdown', handlePointerDown);
	document.addEventListener('pointermove', handlePointerMove);
	document.addEventListener('pointerup', handlePointerUp);
	document.addEventListener('pointercancel', handlePointerUp);
	options.stageElement.addEventListener('click', handleClick, true);
	options.stageElement.addEventListener('dblclick', handleDoubleClick, true);

	return () => {
		options.stageElement.removeEventListener('pointerdown', handlePointerDown);
		document.removeEventListener('pointermove', handlePointerMove);
		document.removeEventListener('pointerup', handlePointerUp);
		document.removeEventListener('pointercancel', handlePointerUp);
		options.stageElement.removeEventListener('click', handleClick, true);
		options.stageElement.removeEventListener('dblclick', handleDoubleClick, true);
	};
}

export function monthRangePreviewSegmentsFromSelection(
	stageElement: HTMLElement | null,
	selection: MonthRangeSelection | null
): MonthRangePreviewSegment[] {
	if (!stageElement) return [];
	const selectedRange = selectedMonthRange(selection);
	if (!selectedRange) return [];
	const stageRectangle = stageElement.getBoundingClientRect();
	const selectedDateCells = selectedMonthDateCells(stageElement, selectedRange);
	const dateCellsByWeek = new Map<HTMLElement, HTMLElement[]>();
	for (const dateCell of selectedDateCells) {
		const weekElement = dateCell.closest('.df-month-week-grid');
		if (!(weekElement instanceof HTMLElement)) continue;
		dateCellsByWeek.set(weekElement, [...(dateCellsByWeek.get(weekElement) ?? []), dateCell]);
	}
	return Array.from(dateCellsByWeek.entries()).flatMap(([weekElement, dateCells], index) =>
		monthRangePreviewSegmentFromCells(weekElement, dateCells, stageRectangle, index)
	);
}

export function orderedDateKeys(firstDateKey: string, secondDateKey: string): [string, string] {
	if (firstDateKey <= secondDateKey) return [firstDateKey, secondDateKey];
	return [secondDateKey, firstDateKey];
}

function hasPointerMoved(selection: MonthRangeSelection, event: PointerEvent): boolean {
	return Math.hypot(event.clientX - selection.startClientX, event.clientY - selection.startClientY) > 8;
}

function monthDateCellFromEventTarget(options: MonthRangeActionOptions, target: EventTarget | null): MonthDateCell | null {
	if (options.currentView() !== ViewType.MONTH) return null;
	if (!(target instanceof Element)) return null;
	if (isMonthRangeIgnoredTarget(target)) return null;
	return monthDateCellFromElement(target);
}

function monthDateCellFromPoint(clientX: number, clientY: number): MonthDateCell | null {
	const element = document.elementFromPoint(clientX, clientY);
	if (!element) return null;
	return monthDateCellFromElement(element);
}

function monthDateCellFromElement(element: Element): MonthDateCell | null {
	const dateCell = element.closest('.df-month-day-cell[data-date]');
	if (!(dateCell instanceof HTMLElement)) return null;
	const dateKey = dateCell.dataset.date;
	if (!dateKey) return null;
	return { element: dateCell, dateKey };
}

function monthDateAnchorFromPoint(clientX: number, clientY: number): DraftPopoverAnchor {
	const dateCell = monthDateCellFromPoint(clientX, clientY);
	if (dateCell) return monthDateAnchorFromCell(dateCell.element);
	return {
		clientX,
		clientY,
		leftClientX: clientX - 8,
		topClientY: clientY - 8,
		bottomClientY: clientY + 8
	};
}

function monthDateAnchorFromCell(element: HTMLElement): DraftPopoverAnchor {
	const rectangle = element.getBoundingClientRect();
	return {
		clientX: rectangle.right,
		clientY: rectangle.top + Math.min(48, rectangle.height / 2),
		leftClientX: rectangle.left,
		topClientY: rectangle.top,
		bottomClientY: rectangle.bottom
	};
}

function isMonthRangeIgnoredTarget(target: Element): boolean {
	return Boolean(
		target.closest(
			'.df-event, .df-month-more-events, .df-event-detail-panel, .df-dialog-container, [data-range-picker-popup], [data-calendar-picker-dropdown]'
		)
	);
}

function selectedMonthRange(selection: MonthRangeSelection | null): { startDateKey: string; endDateKey: string } | null {
	if (!selection || !selection.hasMoved) return null;
	const [startDateKey, endDateKey] = orderedDateKeys(selection.startDateKey, selection.endDateKey);
	return { startDateKey, endDateKey };
}

function selectedMonthDateCells(stageElement: HTMLElement, selectedRange: { startDateKey: string; endDateKey: string }): HTMLElement[] {
	return Array.from(stageElement.querySelectorAll('.df-month-day-cell[data-date]')).filter(
		(element): element is HTMLElement => isSelectedMonthDateCell(element, selectedRange)
	);
}

function isSelectedMonthDateCell(element: Element, selectedRange: { startDateKey: string; endDateKey: string }): boolean {
	if (!(element instanceof HTMLElement)) return false;
	const dateKey = element.dataset.date ?? '';
	return dateKey >= selectedRange.startDateKey && dateKey <= selectedRange.endDateKey;
}

function monthRangePreviewSegmentFromCells(
	weekElement: HTMLElement,
	dateCells: HTMLElement[],
	stageRectangle: DOMRect,
	index: number
): MonthRangePreviewSegment[] {
	const sortedDateCells = [...dateCells].sort((firstDateCell, secondDateCell) =>
		(firstDateCell.dataset.date ?? '').localeCompare(secondDateCell.dataset.date ?? '')
	);
	const firstDateCell = sortedDateCells[0];
	const lastDateCell = sortedDateCells[sortedDateCells.length - 1];
	if (!firstDateCell || !lastDateCell) return [];
	const firstDateCellRectangle = firstDateCell.getBoundingClientRect();
	const lastDateCellRectangle = lastDateCell.getBoundingClientRect();
	const weekRectangle = weekElement.getBoundingClientRect();
	return [
		{
			id: `${firstDateCell.dataset.date ?? index}-${lastDateCell.dataset.date ?? index}`,
			left: firstDateCellRectangle.left - stageRectangle.left + 4,
			top: weekRectangle.top - stageRectangle.top + 34,
			width: Math.max(28, lastDateCellRectangle.right - firstDateCellRectangle.left - 8),
			height: 18
		}
	];
}
