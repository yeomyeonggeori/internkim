import {
	addCalendarGridDays,
	calendarGridWeek,
	startOfCalendarGridDay,
	startOfCalendarGridMonth,
	startOfCalendarGridWeek,
	type CalendarGridWeek
} from './calendar-grid-dates';
import type { CalendarGridEvent } from './calendar-grid-layout';

export function calendarGridMonthWeeks(date: Date): CalendarGridWeek[] {
	const monthStart = startOfCalendarGridMonth(date);
	const nextMonthStart = new Date(monthStart.getFullYear(), monthStart.getMonth() + 1, 1);
	const weeks: CalendarGridWeek[] = [];
	for (let weekStart = startOfCalendarGridWeek(monthStart); weekStart < nextMonthStart; weekStart = addCalendarGridDays(weekStart, 7)) {
		weeks.push(calendarGridWeek(weekStart));
	}
	return weeks;
}

export function calendarGridEventsOnDay(events: CalendarGridEvent[], day: Date): CalendarGridEvent[] {
	const dayStart = startOfCalendarGridDay(day);
	const dayEnd = addCalendarGridDays(dayStart, 1);
	return events
		.filter((event) => event.start < dayEnd && (event.end > dayStart || event.start >= dayStart))
		.sort(compareAgendaEvents);
}

export function shiftedCalendarGridMonthDay(date: Date, direction: -1 | 1): Date {
	const targetMonthStart = new Date(date.getFullYear(), date.getMonth() + direction, 1);
	const lastDayOfTargetMonth = new Date(targetMonthStart.getFullYear(), targetMonthStart.getMonth() + 1, 0).getDate();
	return new Date(targetMonthStart.getFullYear(), targetMonthStart.getMonth(), Math.min(date.getDate(), lastDayOfTargetMonth));
}

function compareAgendaEvents(first: CalendarGridEvent, second: CalendarGridEvent): number {
	return (
		(first.displayPriority ?? 1) - (second.displayPriority ?? 1) ||
		Number(second.isAllDay) - Number(first.isAllDay) ||
		first.start.getTime() - second.start.getTime()
	);
}
