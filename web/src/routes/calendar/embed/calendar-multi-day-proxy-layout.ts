import type { CalendarViewType, Event as DayFlowEvent } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import {
	dayFlowSelector,
	dayFlowTimedEventElements,
	dayFlowWeekAllDayCells
} from './calendar-dayflow-dom-adapter';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';
import {
	calendarMultiDayProxyClass,
	removeUnusedCalendarMultiDayProxyElements,
	reusableCalendarMultiDayProxyElements,
	syncCalendarMultiDayProxyElements,
	type CalendarMultiDayProxyElementLayout,
	type ReusableCalendarMultiDayProxyElements
} from './calendar-multi-day-proxy-elements';

type CalendarMultiDayProxyLayoutContext = {
	stageElement: HTMLElement | null;
	currentView: CalendarViewType;
	currentDate: Date;
	events: DayFlowEvent[];
	selectedEventID: string | null;
};

type ProxySegment = {
	event: DayFlowEvent;
	startDate: Date;
	endDate: Date;
	rowIndex: number;
};

const hiddenRegularClass = 'calendar-multi-day-regular-hidden';
const proxyTop = 2;
const compactProxyWidth = 768;
const compactProxyHeight = 24;
const desktopProxyHeight = 16;
const proxyGap = 4;
const proxyInset = 4;

export function syncCalendarMultiDayProxyLayout(context: CalendarMultiDayProxyLayoutContext): void {
	const { stageElement } = context;
	if (!stageElement) return;
	const proxyHeight = stageElement.getBoundingClientRect().width < compactProxyWidth ? compactProxyHeight : desktopProxyHeight;
	const reusableProxyElements = reusableCalendarMultiDayProxyElements(stageElement);
	if (context.currentView === ViewType.WEEK) {
		syncWeekMultiDayProxyLayout(context, reusableProxyElements, proxyHeight);
	} else if (context.currentView === ViewType.DAY) {
		syncDayMultiDayProxyLayout(context, reusableProxyElements, proxyHeight);
	} else {
		clearCalendarMultiDayRegularLayout(stageElement);
	}
	removeUnusedCalendarMultiDayProxyElements(reusableProxyElements);
}

export function clearCalendarMultiDayProxyLayout(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	for (const proxyElement of stageElement.querySelectorAll<HTMLElement>(`.${calendarMultiDayProxyClass}`)) {
		proxyElement.remove();
	}
	clearCalendarMultiDayRegularLayout(stageElement);
}

function clearCalendarMultiDayRegularLayout(stageElement: HTMLElement): void {
	syncRegularMultiDayVisibility(stageElement, []);
	syncProxyRowCount(stageElement, null);
}

function syncWeekMultiDayProxyLayout(
	context: CalendarMultiDayProxyLayoutContext,
	reusableProxyElements: ReusableCalendarMultiDayProxyElements,
	proxyHeight: number
): void {
	const rowElement = context.stageElement?.querySelector<HTMLElement>(dayFlowSelector.weekAllDayRow);
	if (!rowElement) {
		if (context.stageElement) clearCalendarMultiDayRegularLayout(context.stageElement);
		return;
	}
	const layerElement = context.stageElement?.querySelector<HTMLElement>(dayFlowSelector.weekAllDayEventLayer) ?? rowElement;
	const cells = dayFlowWeekAllDayCells(rowElement);
	if (cells.length === 0) {
		if (context.stageElement) clearCalendarMultiDayRegularLayout(context.stageElement);
		return;
	}
	const visibleStartDate = startOfWeek(context.currentDate);
	const visibleEndDate = addDays(visibleStartDate, cells.length - 1);
	const segments = multiDayProxySegments(context.events, visibleStartDate, visibleEndDate);
	const layerRectangle = layerElement.getBoundingClientRect();
	const cellRectangles = cells.map((cell) => cell.getBoundingClientRect());
	const layouts = segments.flatMap<CalendarMultiDayProxyElementLayout>((segment) => {
		const firstDayIndex = Math.max(0, localDayDiff(visibleStartDate, segment.startDate));
		const lastDayIndex = Math.min(cells.length - 1, localDayDiff(visibleStartDate, segment.endDate));
		const firstCellRectangle = cellRectangles[firstDayIndex];
		const lastCellRectangle = cellRectangles[lastDayIndex];
		if (!firstCellRectangle || !lastCellRectangle) return [];
		const left = firstCellRectangle.left - layerRectangle.left + proxyInset;
		const width = lastCellRectangle.right - firstCellRectangle.left - proxyInset * 2;
		return [proxyLayout(segment, left, width, segment.event.id === context.selectedEventID, proxyHeight)];
	});
	syncRegularMultiDayVisibility(context.stageElement, segments);
	syncCalendarMultiDayProxyElements(layerElement, layouts, reusableProxyElements);
	if (context.stageElement) syncProxyRowCount(context.stageElement, layouts.length);
}

function syncDayMultiDayProxyLayout(
	context: CalendarMultiDayProxyLayoutContext,
	reusableProxyElements: ReusableCalendarMultiDayProxyElements,
	proxyHeight: number
): void {
	const layerElement = context.stageElement?.querySelector<HTMLElement>(dayFlowSelector.dayAllDayLane);
	if (!layerElement) {
		if (context.stageElement) clearCalendarMultiDayRegularLayout(context.stageElement);
		return;
	}
	const visibleDate = startOfDay(context.currentDate);
	const segments = multiDayProxySegments(context.events, visibleDate, visibleDate);
	const layerRectangle = layerElement.getBoundingClientRect();
	const layouts = segments.map((segment) =>
		proxyLayout(
			segment,
			proxyInset,
			layerRectangle.width - proxyInset * 2,
			segment.event.id === context.selectedEventID,
			proxyHeight
		)
	);
	syncRegularMultiDayVisibility(context.stageElement, segments);
	syncCalendarMultiDayProxyElements(layerElement, layouts, reusableProxyElements);
	if (context.stageElement) syncProxyRowCount(context.stageElement, layouts.length);
}

function multiDayProxySegments(events: DayFlowEvent[], visibleStartDate: Date, visibleEndDate: Date): ProxySegment[] {
	return events
		.filter(isMultiDayTimedEvent)
		.filter((event) => rangesOverlap(displayStartDate(event), displayEndDate(event), visibleStartDate, visibleEndDate))
		.sort(compareProxyEvents)
		.map((event, rowIndex) => ({
			event,
			startDate: displayStartDate(event),
			endDate: displayEndDate(event),
			rowIndex
			}));
}

function syncRegularMultiDayVisibility(stageElement: HTMLElement | null, segments: ProxySegment[]): void {
	if (!stageElement) return;
	const eventIDs = new Set(segments.map((segment) => segment.event.id));
	for (const eventElement of dayFlowTimedEventElements(stageElement)) {
		const eventID = eventElement.dataset.eventId;
		const shouldHide = Boolean(eventID && eventIDs.has(eventID));
		if (eventElement.classList.contains(hiddenRegularClass) === shouldHide) continue;
		eventElement.classList.toggle(hiddenRegularClass, shouldHide);
	}
}

function proxyLayout(
	segment: ProxySegment,
	left: number,
	width: number,
	isSelected: boolean,
	proxyHeight: number
): CalendarMultiDayProxyElementLayout {
	return {
		eventID: `${segment.event.id}::multi-day-proxy`,
		height: proxyHeight,
		isSelected,
		label: `${segment.event.title} ${formatEventTime(eventStartDate(segment.event))}`,
		left: Math.round(left),
		top: proxyTop + segment.rowIndex * (proxyHeight + proxyGap),
		width: Math.max(28, Math.round(width))
	};
}

function syncProxyRowCount(stageElement: HTMLElement, rowCount: number | null): void {
	const property = '--calendar-multi-day-all-day-rows';
	if (rowCount === null) {
		if (stageElement.style.getPropertyValue(property)) stageElement.style.removeProperty(property);
		return;
	}
	const value = String(rowCount);
	if (stageElement.style.getPropertyValue(property) !== value) stageElement.style.setProperty(property, value);
}

function isMultiDayTimedEvent(event: DayFlowEvent): boolean {
	if (event.allDay) return false;
	return localDayDiff(displayStartDate(event), displayEndDate(event)) > 0;
}

function displayStartDate(event: DayFlowEvent): Date {
	return startOfDay(eventStartDate(event));
}

function displayEndDate(event: DayFlowEvent): Date {
	const startDate = eventStartDate(event);
	const endDate = eventEndDate(event);
	if (!isLocalMidnight(endDate) || endDate.getTime() <= startDate.getTime()) return startOfDay(endDate);
	return addDays(startOfDay(endDate), -1);
}

function rangesOverlap(startDate: Date, endDate: Date, visibleStartDate: Date, visibleEndDate: Date): boolean {
	return startDate <= visibleEndDate && visibleStartDate <= endDate;
}

function compareProxyEvents(firstEvent: DayFlowEvent, secondEvent: DayFlowEvent): number {
	const startDiff = eventStartDate(firstEvent).getTime() - eventStartDate(secondEvent).getTime();
	if (startDiff !== 0) return startDiff;
	return firstEvent.title.localeCompare(secondEvent.title);
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

function localDayDiff(startDate: Date, targetDate: Date): number {
	return Math.round((startOfDay(targetDate).getTime() - startOfDay(startDate).getTime()) / (24 * 60 * 60 * 1000));
}

function isLocalMidnight(date: Date): boolean {
	return date.getHours() === 0 && date.getMinutes() === 0 && date.getSeconds() === 0 && date.getMilliseconds() === 0;
}

function formatEventTime(date: Date): string {
	return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}
