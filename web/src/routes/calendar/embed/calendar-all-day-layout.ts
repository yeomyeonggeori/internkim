import { ViewType } from '@dayflow/svelte';
import type { CalendarViewType } from '@dayflow/core';
import { dayFlowSelector, visibleDayFlowElements } from './calendar-dayflow-dom-adapter';

const emptyDayAllDayRowHeight = 48;
const compactAllDayRowHeight = 36;
const compactDayAllDayRowHeight = 72;
const allDayEventHeight = 16;
const allDayEventGap = 4;
const allDaySingleEventTop = 7;
const allDayStackEventTop = 2;

export function syncCalendarAllDayLayout(stageElement: HTMLElement | null, currentView: CalendarViewType): void {
	if (!stageElement) return;
	if (currentView === ViewType.DAY) {
		syncDayAllDayLayout(stageElement);
		clearWeekAllDayLayout(stageElement);
		return;
	}
	if (currentView === ViewType.WEEK) {
		syncWeekAllDayLayout(stageElement);
		clearDayAllDayLayout(stageElement);
		return;
	}
	clearCalendarAllDayLayout(stageElement);
}

export function clearCalendarAllDayLayout(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	clearDayAllDayLayout(stageElement);
	clearWeekAllDayLayout(stageElement);
}

function syncDayAllDayLayout(stageElement: HTMLElement): void {
	const rowElement = stageElement.querySelector<HTMLElement>(dayFlowSelector.dayAllDayRow);
	if (!rowElement) return;
	const eventElements = visibleDayFlowElements(rowElement, `${dayFlowSelector.dayAllDayLane} .df-event, ${dayFlowSelector.multiDayProxy}`);
	const rowCount = eventElements.length;
	const rowHeight = dayAllDayRowHeight(rowCount);
	rowElement.style.setProperty('--calendar-day-all-day-row-height', `${rowHeight}px`);
	rowElement.dataset.eventRows = String(rowCount);
	eventElements.forEach((eventElement, index) => {
		applyAllDayEventGeometry(eventElement, rowCount, index);
	});
}

function syncWeekAllDayLayout(stageElement: HTMLElement): void {
	const rowElement = stageElement.querySelector<HTMLElement>(dayFlowSelector.weekAllDayRow);
	if (!rowElement) return;
	const eventElements = visibleDayFlowElements(stageElement, `${dayFlowSelector.weekAllDayEventLayer} .df-event, ${dayFlowSelector.multiDayProxy}`);
	const compactEventRowIndexes = compactAllDayEventRowIndexes(rowElement, eventElements);
	const rowCount = allDayEventRowCount(compactEventRowIndexes);
	const rowHeight = allDayRowHeight(rowCount);
	stageElement.style.setProperty('--calendar-week-all-day-row-content-height', `${rowHeight}px`);
	rowElement.dataset.eventRows = String(rowCount);
	eventElements.forEach((eventElement) => {
		const rowIndex = compactEventRowIndexes.get(eventElement) ?? 0;
		applyAllDayEventGeometry(eventElement, rowCount, rowIndex);
	});
}

function clearDayAllDayLayout(stageElement: HTMLElement): void {
	const rowElement = stageElement.querySelector<HTMLElement>(dayFlowSelector.dayAllDayRow);
	if (!rowElement) return;
	rowElement.style.removeProperty('--calendar-day-all-day-row-height');
	delete rowElement.dataset.eventRows;
	for (const eventElement of visibleDayFlowElements(rowElement, `${dayFlowSelector.dayAllDayLane} .df-event, ${dayFlowSelector.multiDayProxy}`)) {
		clearAllDayEventGeometry(eventElement);
	}
}

function clearWeekAllDayLayout(stageElement: HTMLElement): void {
	stageElement.style.removeProperty('--calendar-week-all-day-row-content-height');
	for (const rowElement of stageElement.querySelectorAll<HTMLElement>(dayFlowSelector.weekAllDayRow)) {
		delete rowElement.dataset.eventRows;
	}
	for (const eventElement of visibleDayFlowElements(stageElement, `${dayFlowSelector.weekAllDayEventLayer} .df-event, ${dayFlowSelector.multiDayProxy}`)) {
		clearAllDayEventGeometry(eventElement);
	}
}

function allDayEventRowIndex(rowElement: HTMLElement, eventElement: HTMLElement): number {
	const rowRectangle = rowElement.getBoundingClientRect();
	const eventRectangle = eventElement.getBoundingClientRect();
	const rowStride = allDayEventHeight + allDayEventGap;
	return Math.max(0, Math.round((eventRectangle.top - rowRectangle.top - allDayStackEventTop) / rowStride));
}

function compactAllDayEventRowIndexes(rowElement: HTMLElement, eventElements: HTMLElement[]): Map<HTMLElement, number> {
	const eventPlacements = eventElements.map((eventElement) => ({
		eventElement,
		rowIndex: allDayEventRowIndex(rowElement, eventElement),
		...allDayEventHorizontalSpan(eventElement)
	})).sort(compareAllDayEventPlacements);
	const rowRightEdges: number[] = [];
	const rowIndexByEventElement = new Map<HTMLElement, number>();
	for (const placement of eventPlacements) {
		const rowIndex = firstAvailableAllDayRowIndex(rowRightEdges, placement.left);
		rowRightEdges[rowIndex] = placement.right;
		rowIndexByEventElement.set(placement.eventElement, rowIndex);
	}
	return rowIndexByEventElement;
}

function allDayEventRowCount(eventRowIndexes: Map<HTMLElement, number>): number {
	if (eventRowIndexes.size === 0) return 0;
	return Math.max(...eventRowIndexes.values()) + 1;
}

function allDayEventHorizontalSpan(eventElement: HTMLElement): { left: number; right: number } {
	const rectangle = eventElement.getBoundingClientRect();
	return {
		left: rectangle.left,
		right: rectangle.right
	};
}

function compareAllDayEventPlacements(
	firstPlacement: { rowIndex: number; left: number; right: number },
	secondPlacement: { rowIndex: number; left: number; right: number }
): number {
	if (firstPlacement.rowIndex !== secondPlacement.rowIndex) return firstPlacement.rowIndex - secondPlacement.rowIndex;
	if (firstPlacement.left !== secondPlacement.left) return firstPlacement.left - secondPlacement.left;
	return secondPlacement.right - firstPlacement.right;
}

function firstAvailableAllDayRowIndex(rowRightEdges: number[], eventLeft: number): number {
	const overlapTolerance = 1;
	const rowIndex = rowRightEdges.findIndex((rowRightEdge) => rowRightEdge <= eventLeft + overlapTolerance);
	return rowIndex >= 0 ? rowIndex : rowRightEdges.length;
}

function allDayRowHeight(rowCount: number): number {
	if (rowCount <= 1) return compactAllDayRowHeight;
	return Math.max(compactAllDayRowHeight, allDayStackEventTop + rowCount * (allDayEventHeight + allDayEventGap) + allDayEventGap);
}

function dayAllDayRowHeight(rowCount: number): number {
	if (rowCount === 0) return emptyDayAllDayRowHeight;
	if (rowCount === 1) return compactDayAllDayRowHeight;
	return Math.max(compactDayAllDayRowHeight, allDayRowHeight(rowCount));
}

function allDayEventTop(rowCount: number, rowIndex: number): number {
	if (rowCount <= 1) return allDaySingleEventTop;
	return allDayStackEventTop + rowIndex * (allDayEventHeight + allDayEventGap);
}

function applyAllDayEventGeometry(eventElement: HTMLElement, rowCount: number, rowIndex: number): void {
	eventElement.style.top = `${allDayEventTop(rowCount, rowIndex)}px`;
	eventElement.style.height = `${allDayEventHeight}px`;
	eventElement.style.minHeight = `${allDayEventHeight}px`;
	eventElement.style.lineHeight = `${allDayEventHeight}px`;
}

function clearAllDayEventGeometry(eventElement: HTMLElement): void {
	eventElement.style.removeProperty('top');
	eventElement.style.removeProperty('height');
	eventElement.style.removeProperty('min-height');
	eventElement.style.removeProperty('line-height');
}
