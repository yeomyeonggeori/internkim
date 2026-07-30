import { ViewType, type ViewType as CalendarViewType } from '../calendar-view-type';
import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';
import {
	calendarTimelineDisplayDayCount,
	calendarTimelineDisplayStartDate
} from './calendar-mobile-two-day-week';
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
	isMobileTwoDayWeekView: boolean;
	events: DayFlowEvent[];
};

type TimelineLayout = {
	gridLeft: number;
	gridTop: number;
	columnWidth: number;
	minuteHeight: number;
	boundaryTop: number;
	viewportTop: number;
	viewportBottom: number;
};

const minimumPreviewHeight = 24;
const minimumTimelinePreviewWidth = 18;
const timelinePreviewColumnInset = 4;
const timelinePreviewLaneGap = 8;
const defaultHourHeight = 72;
const defaultBoundaryTop = 12;

export function timelineRangePreviewSegments(context: TimelineRangePreviewContext): TimelineRangePreviewSegment[] {
	if (!context.stageElement || !context.selection?.hasMoved) return [];
	const layout = timelineLayout(context.stageElement, context.currentView, context.isMobileTwoDayWeekView);
	if (!layout) return [];
	const [startDate, endDate] = orderedTimelineRangeDates(context.selection.startDate, context.selection.currentDate);
	const displayStartDate = calendarTimelineDisplayStartDate(
		context.currentDate,
		context.currentView,
		context.isMobileTwoDayWeekView
	);
	const displayDays = calendarTimelineDisplayDayCount(context.currentView, context.isMobileTwoDayWeekView);
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

export function orderedTimelineRangeDates(firstDate: Date, secondDate: Date): [Date, Date] {
	const startDate = new Date(Math.min(firstDate.getTime(), secondDate.getTime()));
	const endDate = new Date(Math.max(firstDate.getTime(), secondDate.getTime()));
	if (endDate.getTime() - startDate.getTime() < 30 * 60 * 1000) {
		endDate.setTime(startDate.getTime() + 30 * 60 * 1000);
	}
	return [startDate, endDate];
}

export function timelineEventsOverlappingRange(events: DayFlowEvent[], startDate: Date, endDate: Date): DayFlowEvent[] {
	return events.filter((event) => {
		if (event.allDay || event.id.startsWith('timeline-')) return false;
		const candidateStartDate = eventStartDate(event);
		const candidateEndDate = eventEndDate(event);
		return candidateStartDate < endDate && startDate < candidateEndDate;
	});
}

export function timelineOverlapWidthPercent(laneCount: number): number {
	return Math.max(18, 100 / laneCount - 1.5);
}

export function timelinePreviewOverlapWidth(baseWidthPx: number, laneCount: number): number {
	return Math.min(baseWidthPx, Math.max(minimumTimelinePreviewWidth, (baseWidthPx + timelinePreviewLaneGap) / laneCount - timelinePreviewLaneGap));
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
	const availableColumnWidth = Math.max(minimumTimelinePreviewWidth, layout.columnWidth - timelinePreviewColumnInset * 2);
	const availableLaneWidth = Math.max(minimumTimelinePreviewWidth, availableColumnWidth / laneCount - timelinePreviewLaneGap);
	const laneWidth = Math.min(availableColumnWidth, Math.max(minimumTimelinePreviewWidth, availableLaneWidth));
	const left = layout.gridLeft + columnIndex * layout.columnWidth + timelinePreviewColumnInset;
	const unclippedTop = layout.gridTop + layout.boundaryTop + startMinute * layout.minuteHeight;
	const unclippedHeight = Math.max(minimumPreviewHeight, (endMinute - startMinute) * layout.minuteHeight);
	const top = Math.max(unclippedTop, layout.viewportTop);
	const bottom = Math.min(unclippedTop + unclippedHeight, layout.viewportBottom);
	if (bottom <= top) return null;
	return {
		id: `timeline-preview-${dateKey(cursorDate)}`,
		left,
		top,
		width: laneWidth,
		height: bottom - top,
		timeLabel: `${timeLabelFromMinute(startMinute)} - ${timeLabelFromMinute(endMinute)}`,
		isOverlapped: overlapCount > 0,
		overlapLeft: '0%',
		overlapWidth: `${timelineOverlapWidthPercent(laneCount)}%`
	};
}

function timelineLayout(
	stageElement: HTMLElement,
	view: CalendarViewType,
	isMobileTwoDayWeekView: boolean
): TimelineLayout | null {
	const gridElement = timelineGridElement(stageElement, view);
	if (!gridElement) return null;
	const displayDays = calendarTimelineDisplayDayCount(view, isMobileTwoDayWeekView);
	const stageRectangle = stageElement.getBoundingClientRect();
	const gridRectangle = gridElement.getBoundingClientRect();
	const viewportRectangle = (timelineViewportElement(stageElement, view) ?? gridElement).getBoundingClientRect();
	const hourHeight = stageElement.querySelector<HTMLElement>('.df-time-slot')?.getBoundingClientRect().height || defaultHourHeight;
	return {
		gridLeft: gridRectangle.left - stageRectangle.left,
		gridTop: gridRectangle.top - stageRectangle.top,
		columnWidth: gridRectangle.width / displayDays,
		minuteHeight: hourHeight / 60,
		boundaryTop: stageElement.querySelector<HTMLElement>('.df-time-grid-boundary-top')?.getBoundingClientRect().height || defaultBoundaryTop,
		viewportTop: viewportRectangle.top - stageRectangle.top,
		viewportBottom: viewportRectangle.bottom - stageRectangle.top
	};
}

function timelineGridElement(stageElement: HTMLElement, view: CalendarViewType): HTMLElement | null {
	const selector = view === ViewType.WEEK ? '.df-week-time-grid-grid' : '.df-day-content-grid-column';
	return stageElement.querySelector<HTMLElement>(selector);
}

function timelineViewportElement(stageElement: HTMLElement, view: CalendarViewType): HTMLElement | null {
	const selector = view === ViewType.WEEK ? '.df-week-time-grid-scroller' : '.df-day-content-grid';
	return stageElement.querySelector<HTMLElement>(selector);
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
