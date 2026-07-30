import { ViewType } from '../calendar-view-type';
import { calendarWeekNavigationDayCount } from './calendar-mobile-two-day-week';

export function shiftedCalendarToolbarDate(
	date: Date,
	viewType: ViewType,
	direction: -1 | 1,
	isMobileTwoDayWeekView = false
): Date {
	const nextDate = new Date(date);
	if (viewType === ViewType.DAY) {
		nextDate.setDate(nextDate.getDate() + direction);
		return nextDate;
	}
	if (viewType === ViewType.WEEK) {
		nextDate.setDate(nextDate.getDate() + direction * calendarWeekNavigationDayCount(isMobileTwoDayWeekView));
		return nextDate;
	}
	nextDate.setMonth(nextDate.getMonth() + direction);
	return nextDate;
}

const monthWindowRadius = 3;

export function startOfMonthWindow(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth() - monthWindowRadius, 1);
}

export function endOfMonthWindow(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth() + monthWindowRadius + 1, 1);
}
