import type { CalendarEvent } from './calendar-layout-types';

export function calendarDateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

export function calendarDateFromKey(value: string): Date {
	const [yearText, monthText, dayText] = value.split('-');
	return new Date(Number(yearText), Number(monthText) - 1, Number(dayText), 12, 0, 0, 0);
}

export function calendarEventDateKeysForMonth(events: CalendarEvent[], monthDate: Date): Set<string> {
	const monthStart = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1);
	const monthEnd = new Date(monthDate.getFullYear(), monthDate.getMonth() + 1, 0);
	const keys = new Set<string>();
	for (const event of events) {
		const startDate = localDateFromISO(event.startISO);
		const rawEndDate = localDateFromISO(event.endISO);
		const endDate = event.isAllDay
			? new Date(rawEndDate.getFullYear(), rawEndDate.getMonth(), rawEndDate.getDate() - 1)
			: rawEndDate;
		const cursor = maxDate(startDate, monthStart);
		const lastDate = minDate(endDate, monthEnd);
		while (cursor <= lastDate) {
			keys.add(calendarDateKey(cursor));
			cursor.setDate(cursor.getDate() + 1);
		}
	}
	return keys;
}

function localDateFromISO(value: string): Date {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return new Date(0);
	return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function maxDate(left: Date, right: Date): Date {
	return new Date(Math.max(left.getTime(), right.getTime()));
}

function minDate(left: Date, right: Date): Date {
	return new Date(Math.min(left.getTime(), right.getTime()));
}
