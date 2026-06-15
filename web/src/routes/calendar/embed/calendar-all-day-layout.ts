// 캘린더 day/week all-day 영역의 높이와 이벤트 stack 위치를 보정합니다.
import { ViewType } from '@dayflow/svelte';
import type { CalendarViewType } from '@dayflow/core';

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
	const rowElement = stageElement.querySelector<HTMLElement>('.df-day-content-all-day-row');
	if (!rowElement) return;
	const eventElements = visibleAllDayElements(rowElement, '.df-day-content-all-day-lane .df-event:not(.calendar-multi-day-all-day-proxy)');
	const rowCount = eventElements.length;
	const rowHeight = dayAllDayRowHeight(rowCount);
	rowElement.style.setProperty('--calendar-day-all-day-row-height', `${rowHeight}px`);
	rowElement.dataset.eventRows = String(rowCount);
	eventElements.forEach((eventElement, index) => {
		applyAllDayEventGeometry(eventElement, rowCount, index);
	});
}

function syncWeekAllDayLayout(stageElement: HTMLElement): void {
	const rowElement = stageElement.querySelector<HTMLElement>('.df-week-all-day-row-content, .df-all-day-row');
	if (!rowElement) return;
	const eventElements = visibleAllDayElements(stageElement, '.df-week-all-day-event-layer .df-event:not(.calendar-multi-day-all-day-proxy)');
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
	const rowElement = stageElement.querySelector<HTMLElement>('.df-day-content-all-day-row');
	if (!rowElement) return;
	rowElement.style.removeProperty('--calendar-day-all-day-row-height');
	delete rowElement.dataset.eventRows;
	for (const eventElement of visibleAllDayElements(rowElement, '.df-day-content-all-day-lane .df-event')) {
		clearAllDayEventGeometry(eventElement);
	}
}

function clearWeekAllDayLayout(stageElement: HTMLElement): void {
	stageElement.style.removeProperty('--calendar-week-all-day-row-content-height');
	for (const rowElement of stageElement.querySelectorAll<HTMLElement>('.df-week-all-day-row-content, .df-all-day-row')) {
		delete rowElement.dataset.eventRows;
	}
	for (const eventElement of visibleAllDayElements(stageElement, '.df-week-all-day-event-layer .df-event')) {
		clearAllDayEventGeometry(eventElement);
	}
}

function visibleAllDayElements(rootElement: HTMLElement, selector: string): HTMLElement[] {
	return Array.from(rootElement.querySelectorAll<HTMLElement>(selector)).filter(isVisibleElement);
}

function isVisibleElement(element: HTMLElement): boolean {
	const rectangle = element.getBoundingClientRect();
	const style = window.getComputedStyle(element);
	return rectangle.width > 0 && rectangle.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
}

function allDayEventRowIndex(rowElement: HTMLElement, eventElement: HTMLElement): number {
	const rowRectangle = rowElement.getBoundingClientRect();
	const eventRectangle = eventElement.getBoundingClientRect();
	const rowStride = allDayEventHeight + allDayEventGap;
	return Math.max(0, Math.round((eventRectangle.top - rowRectangle.top - allDayStackEventTop) / rowStride));
}

function compactAllDayEventRowIndexes(rowElement: HTMLElement, eventElements: HTMLElement[]): Map<HTMLElement, number> {
	const eventRowIndexes = eventElements.map((eventElement) => ({
		eventElement,
		rowIndex: allDayEventRowIndex(rowElement, eventElement)
	}));
	const compactRowIndexByOriginalRowIndex = new Map(
		Array.from(new Set(eventRowIndexes.map(({ rowIndex }) => rowIndex)))
			.sort((leftRowIndex, rightRowIndex) => leftRowIndex - rightRowIndex)
			.map((rowIndex, compactRowIndex) => [rowIndex, compactRowIndex])
	);
	return new Map(
		eventRowIndexes.map(({ eventElement, rowIndex }) => [eventElement, compactRowIndexByOriginalRowIndex.get(rowIndex) ?? 0])
	);
}

function allDayEventRowCount(eventRowIndexes: Map<HTMLElement, number>): number {
	if (eventRowIndexes.size === 0) return 0;
	return Math.max(...eventRowIndexes.values()) + 1;
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
