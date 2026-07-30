import { ViewType } from '../calendar-view-type';
import type { CalendarViewValue } from '../calendar-navigation-message';
import { calendarViewMessageValue } from './calendar-embed-view-helpers';
import { dateFromDateKey } from './calendar-month-selection';
import type { CalendarSearchResult } from './calendar-search';
import { shiftedCalendarToolbarDate } from './calendar-visible-range';

type CalendarPageNavigationContext = {
	getToolbarDate: () => Date;
	getToolbarView: () => ViewType;
	setToolbarView: (viewType: ViewType) => void;
	setVisibleDate: (date: Date) => void;
	setSelectedMonthDateKey: (dateKey: string) => void;
	selectCalendarEvent: (eventID: string) => void;
	broadcastCalendarView: (view: CalendarViewValue) => void;
	isMobileTwoDayWeekView: () => boolean;
};

export type CalendarPageNavigation = {
	changeCalendarView: (viewType: ViewType) => void;
	goToToday: () => void;
	goToPrevious: () => void;
	goToNext: () => void;
	navigateToDateKey: (dateKey: string) => void;
	navigateToSearchResult: (result: CalendarSearchResult) => void;
	setVisibleDate: (date: Date) => void;
};

export function createCalendarPageNavigation(context: CalendarPageNavigationContext): CalendarPageNavigation {
	function shiftedVisibleDate(direction: -1 | 1): Date {
		return shiftedCalendarToolbarDate(
			context.getToolbarDate(),
			context.getToolbarView(),
			direction,
			context.isMobileTwoDayWeekView()
		);
	}

	function selectCalendarDate(date: Date) {
		context.setVisibleDate(date);
	}

	return {
		changeCalendarView: (viewType) => {
			context.setToolbarView(viewType);
			context.broadcastCalendarView(calendarViewMessageValue(viewType));
		},
		goToToday: () => {
			context.setVisibleDate(new Date());
		},
		goToPrevious: () => {
			const visibleDate = shiftedVisibleDate(-1);
			if (context.isMobileTwoDayWeekView()) {
				selectCalendarDate(visibleDate);
				return;
			}
			context.setVisibleDate(visibleDate);
		},
		goToNext: () => {
			const visibleDate = shiftedVisibleDate(1);
			if (context.isMobileTwoDayWeekView()) {
				selectCalendarDate(visibleDate);
				return;
			}
			context.setVisibleDate(visibleDate);
		},
		navigateToDateKey: (dateKey) => {
			const date = dateFromDateKey(dateKey);
			const navigationDate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), 12, 0, 0, 0);
			context.setSelectedMonthDateKey(dateKey);
			selectCalendarDate(navigationDate);
		},
		navigateToSearchResult: (result) => {
			selectCalendarDate(result.startDate);
			context.selectCalendarEvent(result.id);
		},
		setVisibleDate: context.setVisibleDate
	};
}
