import { ViewType } from '@dayflow/svelte';
import type { CalendarViewType, Event as DayFlowEvent } from '@dayflow/core';
import { dayFlowSelector, visibleDayFlowElements } from './calendar-dayflow-dom-adapter';
import {
	calendarDateKey,
	calendarEventDisplayOrderCandidate,
	compareCalendarEventDisplayOrderCandidates,
	type CalendarEventDisplayOrderCandidate
} from './calendar-event-display-order';

type IndexedCalendarEvent = {
	event: DayFlowEvent;
};

type AllDayVisibleRange = {
	startDateKey: string;
	endDateKey: string;
};

type AllDayEventPlacement = {
	eventElement: HTMLElement;
	left: number;
	right: number;
	rowIndex: number;
	displayOrder: CalendarEventDisplayOrderCandidate | null;
};

const emptyDayAllDayRowHeight = 48;
const compactAllDayRowHeight = 36;
const compactDayAllDayRowHeight = 46;
const allDayEventHeight = 16;
const allDayEventGap = 4;
const allDaySingleEventTop = 7;
const allDayStackEventTop = 2;

export function syncCalendarAllDayLayout(
	stageElement: HTMLElement | null,
	currentView: CalendarViewType,
	currentDate: Date,
	events: DayFlowEvent[]
): void {
	if (!stageElement) return;
	if (currentView === ViewType.DAY) {
		syncDayAllDayLayout(stageElement, currentDate, events);
		clearWeekAllDayLayout(stageElement);
		return;
	}
	if (currentView === ViewType.WEEK) {
		syncWeekAllDayLayout(stageElement, currentDate, events);
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

function syncDayAllDayLayout(stageElement: HTMLElement, currentDate: Date, events: DayFlowEvent[]): void {
	const rowElement = stageElement.querySelector<HTMLElement>(dayFlowSelector.dayAllDayRow);
	if (!rowElement) return;
	const eventElements = visibleDayFlowElements(rowElement, `${dayFlowSelector.dayAllDayLane} .df-event`);
	const eventRowIndexes = compactAllDayEventRowIndexes(rowElement, eventElements, events, dayVisibleRange(currentDate));
	const rowCount = allDayEventRowCount(eventRowIndexes);
	const rowHeight = dayAllDayRowHeight(rowCount);
	rowElement.style.setProperty('--calendar-day-all-day-row-height', `${rowHeight}px`);
	rowElement.dataset.eventRows = String(rowCount);
	eventElements.forEach((eventElement) => {
		const rowIndex = eventRowIndexes.get(eventElement) ?? 0;
		applyAllDayEventGeometry(eventElement, rowCount, rowIndex, true);
	});
}

function syncWeekAllDayLayout(stageElement: HTMLElement, currentDate: Date, events: DayFlowEvent[]): void {
	const rowElement = stageElement.querySelector<HTMLElement>(dayFlowSelector.weekAllDayRow);
	if (!rowElement) return;
	const eventElements = visibleDayFlowElements(stageElement, `${dayFlowSelector.weekAllDayEventLayer} .df-event`);
	const compactEventRowIndexes = compactAllDayEventRowIndexes(rowElement, eventElements, events, weekVisibleRange(currentDate));
	const rowCount = allDayEventRowCount(compactEventRowIndexes);
	const rowHeight = allDayRowHeight(rowCount);
	stageElement.style.setProperty('--calendar-week-all-day-row-content-height', `${rowHeight}px`);
	rowElement.dataset.eventRows = String(rowCount);
	eventElements.forEach((eventElement) => {
		const rowIndex = compactEventRowIndexes.get(eventElement) ?? 0;
		applyAllDayEventGeometry(eventElement, rowCount, rowIndex, false);
	});
}

function clearDayAllDayLayout(stageElement: HTMLElement): void {
	const rowElement = stageElement.querySelector<HTMLElement>(dayFlowSelector.dayAllDayRow);
	if (!rowElement) return;
	rowElement.style.removeProperty('--calendar-day-all-day-row-height');
	delete rowElement.dataset.eventRows;
	for (const eventElement of visibleDayFlowElements(rowElement, `${dayFlowSelector.dayAllDayLane} .df-event`)) {
		clearAllDayEventGeometry(eventElement);
	}
}

function clearWeekAllDayLayout(stageElement: HTMLElement): void {
	stageElement.style.removeProperty('--calendar-week-all-day-row-content-height');
	for (const rowElement of stageElement.querySelectorAll<HTMLElement>(dayFlowSelector.weekAllDayRow)) {
		delete rowElement.dataset.eventRows;
	}
	for (const eventElement of visibleDayFlowElements(stageElement, `${dayFlowSelector.weekAllDayEventLayer} .df-event`)) {
		clearAllDayEventGeometry(eventElement);
	}
}

function allDayEventRowIndex(rowElement: HTMLElement, eventElement: HTMLElement): number {
	const rowRectangle = rowElement.getBoundingClientRect();
	const eventRectangle = eventElement.getBoundingClientRect();
	const rowStride = allDayEventHeight + allDayEventGap;
	return Math.max(0, Math.round((eventRectangle.top - rowRectangle.top - allDayStackEventTop) / rowStride));
}

function compactAllDayEventRowIndexes(
	rowElement: HTMLElement,
	eventElements: HTMLElement[],
	events: DayFlowEvent[],
	visibleRange: AllDayVisibleRange
): Map<HTMLElement, number> {
	const indexedEvents = indexedCalendarEvents(events);
	const eventPlacements = eventElements
		.map((eventElement) => allDayEventPlacement(rowElement, eventElement, indexedEvents, visibleRange))
		.sort(compareAllDayEventPlacements);
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
	firstPlacement: AllDayEventPlacement,
	secondPlacement: AllDayEventPlacement
): number {
	if (firstPlacement.displayOrder && secondPlacement.displayOrder) {
		const displayOrderComparison = compareCalendarEventDisplayOrderCandidates(firstPlacement.displayOrder, secondPlacement.displayOrder);
		if (displayOrderComparison !== 0) return displayOrderComparison;
	}
	if (firstPlacement.displayOrder !== secondPlacement.displayOrder) return firstPlacement.displayOrder ? -1 : 1;
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

function allDayEventPlacement(
	rowElement: HTMLElement,
	eventElement: HTMLElement,
	indexedEvents: Map<string, IndexedCalendarEvent>,
	visibleRange: AllDayVisibleRange
): AllDayEventPlacement {
	const eventID = eventIDFromElement(eventElement);
	const indexedEvent = eventID ? indexedEvents.get(eventID) : null;
	return {
		eventElement,
		rowIndex: allDayEventRowIndex(rowElement, eventElement),
		...allDayEventHorizontalSpan(eventElement),
		displayOrder: indexedEvent
			? calendarEventDisplayOrderCandidate(
					indexedEvent.event,
					visibleRange.startDateKey,
					visibleRange.endDateKey
				)
			: null
	};
}

function indexedCalendarEvents(events: DayFlowEvent[]): Map<string, IndexedCalendarEvent> {
	return new Map(events.map((event) => [event.id, { event }]));
}

function eventIDFromElement(eventElement: HTMLElement): string | null {
	const eventID = eventElement.dataset.eventId;
	if (!eventID) return null;
	const [baseEventID = ''] = eventID.split('::');
	return baseEventID || null;
}

function dayVisibleRange(currentDate: Date): AllDayVisibleRange {
	const visibleDateKey = calendarDateKey(startOfDay(currentDate));
	return {
		startDateKey: visibleDateKey,
		endDateKey: visibleDateKey
	};
}

function weekVisibleRange(currentDate: Date): AllDayVisibleRange {
	const startDate = startOfWeek(currentDate);
	return {
		startDateKey: calendarDateKey(startDate),
		endDateKey: calendarDateKey(addDays(startDate, 6))
	};
}

function startOfWeek(date: Date): Date {
	const startDate = startOfDay(date);
	startDate.setDate(startDate.getDate() - startDate.getDay());
	return startDate;
}

function startOfDay(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function addDays(date: Date, days: number): Date {
	const nextDate = new Date(date);
	nextDate.setDate(nextDate.getDate() + days);
	return nextDate;
}

function allDayEventTop(rowCount: number, rowIndex: number, shouldCenterCompactRows: boolean): number {
	if (shouldCenterCompactRows && rowCount > 0 && rowCount <= 2) {
		const groupHeight = rowCount * allDayEventHeight + (rowCount - 1) * allDayEventGap;
		return Math.round((compactDayAllDayRowHeight - groupHeight) / 2) - allDayStackEventTop + rowIndex * (allDayEventHeight + allDayEventGap);
	}
	if (rowCount <= 1) return allDaySingleEventTop;
	return allDayStackEventTop + rowIndex * (allDayEventHeight + allDayEventGap);
}

function applyAllDayEventGeometry(eventElement: HTMLElement, rowCount: number, rowIndex: number, shouldCenterCompactRows: boolean): void {
	eventElement.style.top = `${allDayEventTop(rowCount, rowIndex, shouldCenterCompactRows)}px`;
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
