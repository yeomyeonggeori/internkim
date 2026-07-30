// 캘린더 embed 페이지의 순수 view helper를 제공합니다.
import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import { ViewType } from '../calendar-view-type';
import type { CalendarViewValue } from '../calendar-navigation-message';
import type { MiniCalendarWeekdayLabels } from './calendar-dayflow-mini-calendar-enhancement';

export function formatMonthScrollOverlayText(date: Date, localeCode: string): string {
	return date.toLocaleDateString(localeCode, {
		year: 'numeric',
		month: 'long'
	});
}

export function compareCalendarEventsForDisplay(leftEvent: DayFlowEvent, rightEvent: DayFlowEvent): number {
	const leftPriority = leftEvent.allDay ? 0 : 1;
	const rightPriority = rightEvent.allDay ? 0 : 1;
	if (leftPriority !== rightPriority) return leftPriority - rightPriority;
	return leftEvent.title.localeCompare(rightEvent.title);
}

export function normalizedVisibleDate(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate(), 12, 0, 0, 0);
}

export function isDateInVisibleRange(date: Date, startDate: Date, endDate: Date): boolean {
	return startDate.getTime() <= date.getTime() && date.getTime() <= endDate.getTime();
}

export function calendarViewMessageValue(viewType: ViewType): CalendarViewValue {
	if (viewType === ViewType.DAY) return 'day';
	if (viewType === ViewType.WEEK) return 'week';
	return 'month';
}

export function calendarViewType(view: CalendarViewValue): ViewType {
	if (view === 'day') return ViewType.DAY;
	if (view === 'week') return ViewType.WEEK;
	return ViewType.MONTH;
}

export function miniCalendarWeekdayLabels(locale: string): MiniCalendarWeekdayLabels {
	if (locale === 'ko') return ['일', '월', '화', '수', '목', '금', '토'];
	return ['S', 'M', 'T', 'W', 'T', 'F', 'S'];
}
