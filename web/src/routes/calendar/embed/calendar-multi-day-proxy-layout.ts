import type { CalendarViewType, Event as DayFlowEvent } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { calendarEventAnchorFromElement } from './calendar-event-anchor-capture';
import {
	dayFlowSelector,
	dayFlowTimedEventElements,
	dayFlowWeekAllDayCells
} from './calendar-dayflow-dom-adapter';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';

type CalendarMultiDayProxyLayoutContext = {
	stageElement: HTMLElement | null;
	currentView: CalendarViewType;
	currentDate: Date;
	events: DayFlowEvent[];
	openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
};

type ProxySegment = {
	event: DayFlowEvent;
	startDate: Date;
	endDate: Date;
	rowIndex: number;
};

const proxyClass = 'calendar-multi-day-all-day-proxy';
const proxyStartClass = 'calendar-multi-day-all-day-proxy-start';
const proxyEndClass = 'calendar-multi-day-all-day-proxy-end';
const hiddenRegularClass = 'calendar-multi-day-regular-hidden';
const proxyTop = 2;
const proxyHeight = 16;
const proxyGap = 4;
const proxyInset = 4;

export function syncCalendarMultiDayProxyLayout(context: CalendarMultiDayProxyLayoutContext): void {
	const { stageElement } = context;
	if (!stageElement) return;
	clearCalendarMultiDayProxyLayout(stageElement);
	if (context.currentView === ViewType.WEEK) {
		syncWeekMultiDayProxyLayout(context);
		return;
	}
	if (context.currentView === ViewType.DAY) {
		syncDayMultiDayProxyLayout(context);
	}
}

export function clearCalendarMultiDayProxyLayout(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	for (const proxyElement of stageElement.querySelectorAll<HTMLElement>(`.${proxyClass}`)) {
		proxyElement.remove();
	}
	for (const eventElement of stageElement.querySelectorAll<HTMLElement>(`.${hiddenRegularClass}`)) {
		eventElement.classList.remove(hiddenRegularClass);
	}
	stageElement.style.removeProperty('--calendar-multi-day-all-day-rows');
}

function syncWeekMultiDayProxyLayout(context: CalendarMultiDayProxyLayoutContext): void {
	const rowElement = context.stageElement?.querySelector<HTMLElement>(dayFlowSelector.weekAllDayRow);
	if (!rowElement) return;
	const layerElement = context.stageElement?.querySelector<HTMLElement>(dayFlowSelector.weekAllDayEventLayer) ?? rowElement;
	const cells = dayFlowWeekAllDayCells(rowElement);
	if (cells.length === 0) return;
	const visibleStartDate = startOfWeek(context.currentDate);
	const visibleEndDate = addDays(visibleStartDate, cells.length - 1);
	const segments = multiDayProxySegments(context.events, visibleStartDate, visibleEndDate);
	hideRegularMultiDaySegments(context.stageElement, segments);
	for (const segment of segments) {
		const firstDayIndex = Math.max(0, localDayDiff(visibleStartDate, segment.startDate));
		const lastDayIndex = Math.min(cells.length - 1, localDayDiff(visibleStartDate, segment.endDate));
		const firstCell = cells[firstDayIndex];
		const lastCell = cells[lastDayIndex];
		if (!firstCell || !lastCell) continue;
		const layerRectangle = layerElement.getBoundingClientRect();
		const firstCellRectangle = firstCell.getBoundingClientRect();
		const lastCellRectangle = lastCell.getBoundingClientRect();
		const left = firstCellRectangle.left - layerRectangle.left + proxyInset;
		const width = lastCellRectangle.right - firstCellRectangle.left - proxyInset * 2;
		layerElement.appendChild(createProxyElement(segment, left, width, context.openEvent));
	}
	context.stageElement?.style.setProperty('--calendar-multi-day-all-day-rows', String(segments.length));
}

function syncDayMultiDayProxyLayout(context: CalendarMultiDayProxyLayoutContext): void {
	const layerElement = context.stageElement?.querySelector<HTMLElement>(dayFlowSelector.dayAllDayLane);
	if (!layerElement) return;
	const visibleDate = startOfDay(context.currentDate);
	const segments = multiDayProxySegments(context.events, visibleDate, visibleDate);
	const layerRectangle = layerElement.getBoundingClientRect();
	hideRegularMultiDaySegments(context.stageElement, segments);
	for (const segment of segments) {
		const left = proxyInset;
		const width = layerRectangle.width - proxyInset * 2;
		layerElement.appendChild(createProxyElement(segment, left, width, context.openEvent));
	}
	context.stageElement?.style.setProperty('--calendar-multi-day-all-day-rows', String(segments.length));
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

function hideRegularMultiDaySegments(stageElement: HTMLElement | null, segments: ProxySegment[]): void {
	if (!stageElement) return;
	const eventIDs = new Set(segments.map((segment) => segment.event.id));
	if (eventIDs.size === 0) return;
	for (const eventElement of dayFlowTimedEventElements(stageElement)) {
		const eventID = eventElement.dataset.eventId;
		if (!eventID || !eventIDs.has(eventID)) continue;
		eventElement.classList.add(hiddenRegularClass);
	}
}

function createProxyElement(
	segment: ProxySegment,
	left: number,
	width: number,
	openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void
): HTMLElement {
	const proxyElement = document.createElement('button');
	proxyElement.type = 'button';
	proxyElement.className = `${proxyClass} df-event`;
	proxyElement.dataset.eventId = `${segment.event.id}::multi-day-proxy`;
	proxyElement.style.left = `${Math.round(left)}px`;
	proxyElement.style.top = `${proxyTop + segment.rowIndex * (proxyHeight + proxyGap)}px`;
	proxyElement.style.width = `${Math.max(28, Math.round(width))}px`;
	proxyElement.style.height = `${proxyHeight}px`;
	proxyElement.appendChild(createProxyStartElement(segment));
	proxyElement.appendChild(createProxyEndElement(segment));
	proxyElement.addEventListener('click', (event) => {
		event.preventDefault();
		event.stopPropagation();
		openEvent(segment.event.id, calendarEventAnchorFromElement(proxyElement));
	});
	return proxyElement;
}

function createProxyStartElement(segment: ProxySegment): HTMLElement {
	const startElement = document.createElement('span');
	startElement.className = proxyStartClass;
	startElement.textContent = `${segment.event.title} ${formatEventTime(eventStartDate(segment.event))}`;
	return startElement;
}

function createProxyEndElement(segment: ProxySegment): HTMLElement {
	const endElement = document.createElement('span');
	endElement.className = proxyEndClass;
	endElement.textContent = formatEventTime(eventEndDate(segment.event));
	return endElement;
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
