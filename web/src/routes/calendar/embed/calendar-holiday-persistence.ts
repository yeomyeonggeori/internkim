import type { CalendarHoliday, CalendarHolidayLocale } from '$lib/calendar/holiday';
import { supabaseCalendarHolidays } from '$lib/calendar/supabase-calendar-holidays';

export type { CalendarHoliday, CalendarHolidayLocale };

export type CalendarHolidayLoadResult = {
	holidays: CalendarHoliday[];
	degraded: boolean;
	errorCode?: string;
};

export async function fetchCalendarHolidays(
	startDate: Date,
	endDate: Date,
	locale: CalendarHolidayLocale,
	timeZone?: Promise<string>
): Promise<CalendarHolidayLoadResult> {
	return supabaseCalendarHolidays(startDate, endDate, locale, timeZone);
}
