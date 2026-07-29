import { ViewType } from '@dayflow/svelte';
import type { CalendarViewValue } from '../calendar-navigation-message';
import { calendarViewMessageValue } from './calendar-embed-view-helpers';
import { dateFromDateKey } from './calendar-month-selection';
import type { CalendarSearchResult } from './calendar-search';
import { shiftedCalendarToolbarDate } from './calendar-visible-range';

type CalendarNavigationApp = {
	changeView: (viewType: ViewType) => void;
	goToToday: () => void;
	goToPrevious: () => void;
	goToNext: () => void;
	app: {
		setCurrentDate: (date: Date) => void;
		setVisibleMonth: (date: Date) => void;
		selectDate: (date: Date) => void;
	};
};

type CalendarPageNavigationContext = {
	calendar: CalendarNavigationApp;
	getToolbarDate: () => Date;
	getToolbarView: () => ViewType;
	setToolbarView: (viewType: ViewType) => void;
	setVisibleDate: (date: Date) => void;
	setSelectedMonthDateKey: (dateKey: string) => void;
	refreshSelectedMonthDateCellAfterRender: () => void;
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
		context.calendar.app.setCurrentDate(date);
		context.calendar.app.setVisibleMonth(date);
		context.calendar.app.selectDate(date);
	}

	return {
		changeCalendarView: (viewType) => {
			context.setToolbarView(viewType);
			context.calendar.changeView(viewType);
			context.broadcastCalendarView(calendarViewMessageValue(viewType));
		},
		goToToday: () => {
			context.setVisibleDate(new Date());
			context.calendar.goToToday();
		},
		goToPrevious: () => {
			const visibleDate = shiftedVisibleDate(-1);
			if (context.isMobileTwoDayWeekView()) {
				selectCalendarDate(visibleDate);
				return;
			}
			context.setVisibleDate(visibleDate);
			context.calendar.goToPrevious();
		},
		goToNext: () => {
			const visibleDate = shiftedVisibleDate(1);
			if (context.isMobileTwoDayWeekView()) {
				selectCalendarDate(visibleDate);
				return;
			}
			context.setVisibleDate(visibleDate);
			context.calendar.goToNext();
		},
		navigateToDateKey: (dateKey) => {
			const date = dateFromDateKey(dateKey);
			const navigationDate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), 12, 0, 0, 0);
			context.setSelectedMonthDateKey(dateKey);
			selectCalendarDate(navigationDate);
			context.refreshSelectedMonthDateCellAfterRender();
		},
		navigateToSearchResult: (result) => {
			selectCalendarDate(result.startDate);
			context.selectCalendarEvent(result.id);
		},
		setVisibleDate: context.setVisibleDate
	};
}
