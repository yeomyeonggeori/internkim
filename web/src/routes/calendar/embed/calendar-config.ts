import { registerLocale, type Locale } from '@dayflow/core';
import { createDayView, createMonthView, createWeekView } from '@dayflow/svelte';

export type CalendarDayFlowLocaleText = {
	title: string;
	today: string;
	day: string;
	week: string;
	month: string;
	new: string;
	newEvent: string;
	search: string;
	noResults: string;
	allDay: string;
};

export const calendarColors = {
	eventColor: '#eff6ff',
	eventSelectedColor: 'rgb(37, 99, 235)',
	lineColor: '#3b82f6',
	textColor: '#1e3a8a'
};

export const darkCalendarColors = {
	eventColor: 'rgba(30, 64, 175, 0.8)',
	eventSelectedColor: 'rgba(30, 58, 138, 1)',
	lineColor: '#3b82f6',
	textColor: '#dbeafe'
};

export function createCalendarLocale(locale: 'ko' | 'en', text: CalendarDayFlowLocaleText): Locale {
	const code = locale === 'ko' ? 'ko-KR' : 'en-US';
	const dayFlowLocale = {
		code,
		messages: {
			allDay: text.allDay,
			today: text.today,
			day: text.day,
			week: text.week,
			month: text.month,
			newEvent: text.newEvent,
			titlePlaceholder: text.newEvent,
			quickCreateEvent: text.new,
			quickCreatePlaceholder: text.newEvent,
			search: text.search,
			noResults: text.noResults,
			calendar: text.title
		}
	} satisfies Locale;
	registerLocale(dayFlowLocale);
	return dayFlowLocale;
}

export function createCalendarViews() {
	return [
		createDayView({ showAllDay: true, timeFormat: '24h' }),
		createWeekView({ showWeekends: true, startOfWeek: 0, showAllDay: true }),
		createMonthView({ showWeekNumbers: false, startOfWeek: 0 })
	];
}
