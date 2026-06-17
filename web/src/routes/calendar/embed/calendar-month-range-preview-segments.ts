import type { MonthRangeSelection } from './calendar-month-range-action';

export type MonthRangePreviewSegment = {
	id: string;
	left: number;
	top: number;
	width: number;
	height: number;
};

const monthRangePreviewTopOffset = 34;
const monthRangePreviewEventGap = 2;
const monthRangePreviewInset = 4;
const monthRangePreviewMinimumWidth = 28;

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
		monthRangePreviewSegmentFromCells(stageElement, weekElement, dateCells, stageRectangle, index)
	);
}

export function orderedDateKeys(firstDateKey: string, secondDateKey: string): [string, string] {
	if (firstDateKey <= secondDateKey) return [firstDateKey, secondDateKey];
	return [secondDateKey, firstDateKey];
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
	stageElement: HTMLElement,
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
	const top = monthRangePreviewTop(stageElement, weekRectangle, firstDateCellRectangle.left, lastDateCellRectangle.right, stageRectangle);
	return [
		{
			id: `${firstDateCell.dataset.date ?? index}-${lastDateCell.dataset.date ?? index}`,
			left: firstDateCellRectangle.left - stageRectangle.left + monthRangePreviewInset,
			top,
			width: Math.max(monthRangePreviewMinimumWidth, lastDateCellRectangle.right - firstDateCellRectangle.left - monthRangePreviewInset * 2),
			height: 18
		}
	];
}

function monthRangePreviewTop(
	stageElement: HTMLElement,
	weekRectangle: DOMRect,
	rangeLeft: number,
	rangeRight: number,
	stageRectangle: DOMRect
): number {
	const occupiedBottom = overlappingMonthLayerEventBottom(stageElement, weekRectangle, rangeLeft, rangeRight);
	if (occupiedBottom === null) return weekRectangle.top - stageRectangle.top + monthRangePreviewTopOffset;
	return occupiedBottom - stageRectangle.top + monthRangePreviewEventGap;
}

function overlappingMonthLayerEventBottom(
	stageElement: HTMLElement,
	weekRectangle: DOMRect,
	rangeLeft: number,
	rangeRight: number
): number | null {
	const bottoms = Array.from(stageElement.querySelectorAll<HTMLElement>('.calendar-month-direct-event'))
		.map((eventElement) => eventElement.getBoundingClientRect())
		.filter((eventRectangle) => isVisibleRectangle(eventRectangle))
		.filter((eventRectangle) => eventRectangle.bottom > weekRectangle.top && eventRectangle.top < weekRectangle.bottom)
		.filter((eventRectangle) => eventRectangle.right > rangeLeft && eventRectangle.left < rangeRight)
		.map((eventRectangle) => eventRectangle.bottom);
	if (bottoms.length === 0) return null;
	return Math.max(...bottoms);
}

function isVisibleRectangle(rectangle: DOMRect): boolean {
	return rectangle.width > 0 && rectangle.height > 0;
}
