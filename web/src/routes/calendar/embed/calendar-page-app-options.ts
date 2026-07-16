import type { Event as DayFlowEvent, Locale } from '@dayflow/core';
import { createEventsPlugin, type useCalendarApp, type ViewType } from '@dayflow/svelte';
import { createDragPlugin } from '@dayflow/plugin-drag';
import type { CalendarLocaleText } from '../text';
import { compareCalendarEventsForDisplay, isDateInVisibleRange } from './calendar-embed-view-helpers';
import {
	calendarColors,
	createCalendarViews,
	darkCalendarColors
} from './calendar-config';

type CalendarPageAppOptions = Parameters<typeof useCalendarApp>[0];

type CalendarPageAppOptionsContext = {
	defaultView: ViewType;
	initialDate: Date;
	locale: Locale;
	text: CalendarLocaleText;
	getToolbarDate: () => Date;
	loadEvents: (startDate: Date, endDate: Date) => void;
	setVisibleDate: (date: Date) => void;
	selectCalendarEvent: (eventID: string) => void;
	saveCreatedEvent: (event: DayFlowEvent) => Promise<void>;
	saveUpdatedEvent: (event: DayFlowEvent, previousEvent?: DayFlowEvent) => Promise<void>;
	deleteEvent: (eventID: string) => Promise<void>;
};

export function createCalendarPageAppOptions(context: CalendarPageAppOptionsContext): CalendarPageAppOptions {
	return {
		views: createCalendarViews(),
		defaultView: context.defaultView,
		initialDate: context.initialDate,
		locale: context.locale,
		timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
		switcherMode: 'buttons',
		useCalendarHeader: false,
		useEventDetailDialog: false,
		useEventDetailPanel: false,
		calendars: [
			{
				id: 'internkim',
				name: context.text.work,
				colors: calendarColors,
				darkColors: darkCalendarColors,
				isVisible: true
			}
		],
		defaultCalendar: 'internkim',
		theme: { mode: 'light' },
		allDaySortComparator: compareCalendarEventsForDisplay,
		plugins: [
			createEventsPlugin(),
			createDragPlugin({
				enableDrag: true,
				enableResize: true,
				enableCreate: false,
				enableAllDayCreate: false,
				onEventDrop: (event, previousEvent) => context.saveUpdatedEvent(event, previousEvent),
				onEventResize: (event, previousEvent) => context.saveUpdatedEvent(event, previousEvent)
			})
		],
		callbacks: {
			onVisibleRangeChange: (startDate, endDate) => {
				context.loadEvents(startDate, endDate);
				const middle = new Date((startDate.getTime() + endDate.getTime()) / 2);
				const visibleDate = isDateInVisibleRange(context.getToolbarDate(), startDate, endDate)
					? context.getToolbarDate()
					: middle;
				context.setVisibleDate(visibleDate);
			},
			onEventClick: (event) => context.selectCalendarEvent(event.id),
			onEventCreate: context.saveCreatedEvent,
			onEventUpdate: (event) => context.saveUpdatedEvent(event),
			onEventDelete: context.deleteEvent
		}
	};
}
