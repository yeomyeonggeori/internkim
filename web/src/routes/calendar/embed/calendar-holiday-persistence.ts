import { responseErrorMessage } from './calendar-event-persistence';

export type CalendarHoliday = {
	id: string;
	title: string;
	date: string;
	source: 'holiday_api';
	countryCode?: string;
	readOnly: true;
	color: string;
};

export type CalendarHolidayLocale = 'ko' | 'en';

type CalendarHolidaysResponse = {
	holidays: CalendarHoliday[];
	source: CalendarHoliday['source'];
};

export async function fetchCalendarHolidays(
	startDate: Date,
	endDate: Date,
	locale: CalendarHolidayLocale,
	errorFallback: string
): Promise<CalendarHoliday[]> {
	const query = new URLSearchParams({
		startISO: startDate.toISOString(),
		endISO: endDate.toISOString(),
		locale
	});
	const response = await fetch(`/calendar/api/holidays?${query}`, { credentials: 'include' });
	if (!response.ok) throw new Error(await responseErrorMessage(response, errorFallback));
	const document = (await response.json()) as CalendarHolidaysResponse;
	return document.holidays ?? [];
}
