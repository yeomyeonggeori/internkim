import { ViewType } from '../calendar-view-type';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';

export type MonthDateCell = {
	element: HTMLElement;
	dateKey: string;
};

type MonthRangeHitTestOptions = {
	currentView: () => string;
};

export type MonthRangePointerStart = {
	startClientX: number;
	startClientY: number;
};

export function hasMonthRangePointerMoved(selection: MonthRangePointerStart, event: PointerEvent): boolean {
	return Math.hypot(event.clientX - selection.startClientX, event.clientY - selection.startClientY) > 8;
}

export function monthDateCellFromEventTarget(
	options: MonthRangeHitTestOptions,
	target: EventTarget | null
): MonthDateCell | null {
	if (options.currentView() !== ViewType.MONTH) return null;
	if (!(target instanceof Element)) return null;
	if (isMonthRangeIgnoredTarget(target)) return null;
	return monthDateCellFromElement(target);
}

export function monthDateCellFromPoint(clientX: number, clientY: number): MonthDateCell | null {
	const element = document.elementFromPoint(clientX, clientY);
	if (!element) return null;
	return monthDateCellFromElement(element);
}

export function monthDateAnchorFromPoint(clientX: number, clientY: number): DraftPopoverAnchor {
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

export function monthDateAnchorFromCell(element: HTMLElement): DraftPopoverAnchor {
	const rectangle = element.getBoundingClientRect();
	return {
		clientX: rectangle.right,
		clientY: rectangle.top + Math.min(48, rectangle.height / 2),
		leftClientX: rectangle.left,
		topClientY: rectangle.top,
		bottomClientY: rectangle.bottom
	};
}

function monthDateCellFromElement(element: Element): MonthDateCell | null {
	const dateCell = element.closest('.df-month-day-cell[data-date]');
	if (!(dateCell instanceof HTMLElement)) return null;
	const dateKey = dateCell.dataset.date;
	if (!dateKey) return null;
	return { element: dateCell, dateKey };
}

function isMonthRangeIgnoredTarget(target: Element): boolean {
	return Boolean(
		target.closest(
			'.df-event, .df-month-more-events, .df-event-detail-panel, .df-dialog-container, [data-range-picker-popup], [data-calendar-picker-dropdown]'
		)
	);
}
