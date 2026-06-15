// 캘린더 timeline drag preview segment와 overlap lane 보정을 계산합니다.
import { ViewType } from '@dayflow/svelte';
import type { CalendarViewType, Event as DayFlowEvent } from '@dayflow/core';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';

export type TimelineRangePreviewSegment = {
	id: string;
	left: number;
	top: number;
	width: number;
	height: number;
	timeLabel: string;
	isOverlapped: boolean;
	overlapLeft: string;
	overlapWidth: string;
};

type TimelineRangePreviewContext = {
	stageElement: HTMLElement | null;
	selection: TimelineRangeSelection | null;
	currentView: CalendarViewType;
	currentDate: Date;
	events: DayFlowEvent[];
};

const minimumPreviewHeight = 24;
const minimumLaneWidthPx = 32;
const defaultHourHeight = 72;
const defaultBoundaryTop = 12;

export function timelineRangePreviewSegments(context: TimelineRangePreviewContext): TimelineRangePreviewSegment[] {
	if (!context.stageElement || !context.selection?.hasMoved) return [];
	const layout = timelineLayout(context.stageElement, context.currentView);
	if (!layout) return [];
	const [startDate, endDate] = orderedTimelineRangeDates(context.selection.startDate, context.selection.currentDate);
	const displayStartDate = context.currentView === ViewType.WEEK ? startOfWeek(context.currentDate) : startOfDay(context.currentDate);
	const displayDays = context.currentView === ViewType.WEEK ? 7 : 1;
	const segments: TimelineRangePreviewSegment[] = [];
	const cursorDate = startOfDay(startDate);
	const endDay = startOfDay(endDate);
	while (cursorDate.getTime() <= endDay.getTime()) {
		const columnIndex = localDayDiff(displayStartDate, cursorDate);
		if (columnIndex >= 0 && columnIndex < displayDays) {
			const segment = timelineRangePreviewSegment(context, layout, cursorDate, columnIndex, startDate, endDate);
			if (segment) segments.push(segment);
		}
		cursorDate.setDate(cursorDate.getDate() + 1);
	}
	return segments;
}

export function syncTimelinePreviewOverlapLayout(
	stageElement: HTMLElement | null,
	selection: TimelineRangeSelection | null,
	events: DayFlowEvent[]
): void {
	if (!stageElement) return;
	clearTimelineOverlapLayout(stageElement);
	if (!selection?.hasMoved) return;
	const [startDate, endDate] = orderedTimelineRangeDates(selection.startDate, selection.currentDate);
	const overlappingEvents = timelineEventsOverlappingRange(events, startDate, endDate);
	if (overlappingEvents.length === 0) return;
	const laneCount = overlappingEvents.length + 1;
	const laneWidth = Math.max(18, 100 / laneCount - 1.5);
	for (const element of timelineRangePreviewElements(stageElement)) {
		applyTimelinePreviewOverlapLane(element, laneCount);
	}
	overlappingEvents.forEach((event, index) => {
		for (const element of timelineEventElementsByID(stageElement, event.id)) {
			applyTimelineOverlapLane(element, index + 1, laneCount, laneWidth);
		}
	});
}

export function clearTimelineOverlapLayout(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	for (const element of stageElement.querySelectorAll<HTMLElement>('.calendar-timeline-overlap-adjusted')) {
		restoreTimelinePreviewWidth(element);
		element.classList.remove('calendar-timeline-overlap-adjusted');
		element.style.removeProperty('--calendar-overlap-left');
		element.style.removeProperty('--calendar-overlap-width');
	}
}

function timelineRangePreviewSegment(
	context: TimelineRangePreviewContext,
	layout: TimelineLayout,
	cursorDate: Date,
	columnIndex: number,
	startDate: Date,
	endDate: Date
): TimelineRangePreviewSegment | null {
	const isStartDay = sameCalendarDay(cursorDate, startDate);
	const isEndDay = sameCalendarDay(cursorDate, endDate);
	const startMinute = isStartDay ? minutesSinceMidnight(startDate) : 0;
	const endMinute = isEndDay ? minutesSinceMidnight(endDate) : 24 * 60;
	if (endMinute <= startMinute) return null;
	const segmentStartDate = dateWithMinute(cursorDate, startMinute);
	const segmentEndDate = dateWithMinute(cursorDate, endMinute);
	const overlapCount = timelineEventsOverlappingRange(context.events, segmentStartDate, segmentEndDate).length;
	const laneCount = overlapCount + 1;
	const laneWidth = Math.max(minimumLaneWidthPx, layout.columnWidth / laneCount - 8);
	const left = layout.gridLeft + columnIndex * layout.columnWidth + 4;
	const top = layout.gridTop + layout.boundaryTop + startMinute * layout.minuteHeight;
	return {
		id: `timeline-preview-${dateKey(cursorDate)}`,
		left,
		top,
		width: laneWidth,
		height: Math.max(minimumPreviewHeight, (endMinute - startMinute) * layout.minuteHeight),
		timeLabel: `${timeLabelFromMinute(startMinute)} - ${timeLabelFromMinute(endMinute)}`,
		isOverlapped: overlapCount > 0,
		overlapLeft: '0%',
		overlapWidth: `${Math.max(18, 100 / laneCount - 1.5)}%`
	};
}

type TimelineLayout = {
	gridLeft: number;
	gridTop: number;
	columnWidth: number;
	minuteHeight: number;
	boundaryTop: number;
};

function timelineLayout(stageElement: HTMLElement, view: CalendarViewType): TimelineLayout | null {
	const gridElement = timelineGridElement(stageElement, view);
	if (!gridElement) return null;
	const displayDays = view === ViewType.WEEK ? 7 : 1;
	const stageRectangle = stageElement.getBoundingClientRect();
	const gridRectangle = gridElement.getBoundingClientRect();
	const hourHeight = stageElement.querySelector<HTMLElement>('.df-time-slot')?.getBoundingClientRect().height || defaultHourHeight;
	return {
		gridLeft: gridRectangle.left - stageRectangle.left,
		gridTop: gridRectangle.top - stageRectangle.top,
		columnWidth: gridRectangle.width / displayDays,
		minuteHeight: hourHeight / 60,
		boundaryTop: stageElement.querySelector<HTMLElement>('.df-time-grid-boundary-top')?.getBoundingClientRect().height || defaultBoundaryTop
	};
}

function timelineGridElement(stageElement: HTMLElement, view: CalendarViewType): HTMLElement | null {
	const selector = view === ViewType.WEEK ? '.df-week-time-grid-grid' : '.df-day-content-grid';
	return stageElement.querySelector<HTMLElement>(selector);
}

function timelineEventsOverlappingRange(events: DayFlowEvent[], startDate: Date, endDate: Date): DayFlowEvent[] {
	return events.filter((event) => {
		if (event.allDay || event.id.startsWith('timeline-')) return false;
		const candidateStartDate = eventStartDate(event);
		const candidateEndDate = eventEndDate(event);
		return candidateStartDate < endDate && startDate < candidateEndDate;
	});
}

function timelineEventElementsByID(stageElement: HTMLElement, eventID: string): HTMLElement[] {
	const escapedEventID = window.CSS?.escape(eventID) ?? eventID.replaceAll('"', '\\"');
	return Array.from(stageElement.querySelectorAll<HTMLElement>(`[data-event-id="${escapedEventID}"]`)).filter(
		(element) => !element.closest('.df-right-panel-events, .df-right-panel-calendar-shell, .df-mini-calendar')
	);
}

function timelineRangePreviewElements(stageElement: HTMLElement): HTMLElement[] {
	return Array.from(stageElement.querySelectorAll<HTMLElement>('.timeline-range-preview'));
}

function applyTimelineOverlapLane(element: HTMLElement, laneIndex: number, laneCount: number, laneWidth: number): void {
	element.classList.add('calendar-timeline-overlap-adjusted');
	element.style.setProperty('--calendar-overlap-left', `${(laneIndex * 100) / laneCount}%`);
	element.style.setProperty('--calendar-overlap-width', `${laneWidth}%`);
}

function applyTimelinePreviewOverlapLane(element: HTMLElement, laneCount: number): void {
	const baseWidth = element.dataset.timelinePreviewBaseWidth ?? element.style.width;
	const baseWidthPx = Number.parseFloat(baseWidth);
	if (!Number.isFinite(baseWidthPx)) return;
	element.dataset.timelinePreviewBaseWidth = baseWidth;
	element.classList.add('calendar-timeline-overlap-adjusted');
	element.style.width = `${Math.max(minimumLaneWidthPx, (baseWidthPx + 8) / laneCount - 8)}px`;
}

function restoreTimelinePreviewWidth(element: HTMLElement): void {
	const baseWidth = element.dataset.timelinePreviewBaseWidth;
	if (!baseWidth) return;
	element.style.width = baseWidth;
	delete element.dataset.timelinePreviewBaseWidth;
}

function orderedTimelineRangeDates(firstDate: Date, secondDate: Date): [Date, Date] {
	const startDate = new Date(Math.min(firstDate.getTime(), secondDate.getTime()));
	const endDate = new Date(Math.max(firstDate.getTime(), secondDate.getTime()));
	if (endDate.getTime() - startDate.getTime() < 30 * 60 * 1000) {
		endDate.setTime(startDate.getTime() + 30 * 60 * 1000);
	}
	return [startDate, endDate];
}

function startOfWeek(date: Date): Date {
	const weekStart = startOfDay(date);
	weekStart.setDate(weekStart.getDate() - weekStart.getDay());
	return weekStart;
}

function startOfDay(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function localDayDiff(startDate: Date, targetDate: Date): number {
	const start = startOfDay(startDate).getTime();
	const target = startOfDay(targetDate).getTime();
	return Math.round((target - start) / (24 * 60 * 60 * 1000));
}

function sameCalendarDay(firstDate: Date, secondDate: Date): boolean {
	return dateKey(firstDate) === dateKey(secondDate);
}

function minutesSinceMidnight(date: Date): number {
	return date.getHours() * 60 + date.getMinutes();
}

function dateWithMinute(date: Date, minute: number): Date {
	const nextDate = startOfDay(date);
	nextDate.setHours(Math.floor(minute / 60), minute % 60, 0, 0);
	return nextDate;
}

function timeLabelFromMinute(minute: number): string {
	const normalizedMinute = Math.min(24 * 60, Math.max(0, minute));
	const hour = Math.floor(normalizedMinute / 60);
	const displayHour = hour === 24 ? 24 : hour % 24;
	return `${String(displayHour).padStart(2, '0')}:${String(normalizedMinute % 60).padStart(2, '0')}`;
}

function dateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}
