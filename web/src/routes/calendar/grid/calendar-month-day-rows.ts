import type { CalendarGridDayEntry, CalendarGridSpan, CalendarGridWeekLayout } from './calendar-grid-layout';

export const calendarMonthDayRowCount = 3;
export const calendarMonthVisibleSpanLaneCount = calendarMonthDayRowCount - 1;

export type CalendarMonthDayRows = {
	spanLaneCount: number;
	visibleTimedEntries: CalendarGridDayEntry[];
	hiddenCount: number;
};

export function isCalendarMonthSpanVisible(span: CalendarGridSpan): boolean {
	return span.lane < calendarMonthVisibleSpanLaneCount;
}

export function calendarMonthVisibleLaneCount(layout: CalendarGridWeekLayout): number {
	return Math.min(layout.laneCount, calendarMonthVisibleSpanLaneCount);
}

export function calendarMonthDayRows(layout: CalendarGridWeekLayout, dayIndex: number): CalendarMonthDayRows {
	const daySpans = layout.spans.filter(
		(span) => dayIndex >= span.startColumn && dayIndex < span.startColumn + span.columnCount
	);
	const visibleSpans = daySpans.filter(isCalendarMonthSpanVisible);
	const hiddenSpanCount = daySpans.length - visibleSpans.length;
	const spanLaneCount = visibleSpans.reduce((count, span) => Math.max(count, span.lane + 1), 0);
	const timedEntries = layout.timedEntries.filter((entry) => entry.dayIndex === dayIndex);
	const availableRowCount = calendarMonthDayRowCount - spanLaneCount;
	if (hiddenSpanCount === 0 && timedEntries.length <= availableRowCount) {
		return { spanLaneCount, visibleTimedEntries: timedEntries, hiddenCount: 0 };
	}
	const visibleTimedCount = availableRowCount - 1;
	return {
		spanLaneCount,
		visibleTimedEntries: timedEntries.slice(0, visibleTimedCount),
		hiddenCount: hiddenSpanCount + timedEntries.length - visibleTimedCount
	};
}
