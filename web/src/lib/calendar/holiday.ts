export type CalendarHolidaySource = 'holiday_api' | 'company';

export type CalendarHolidayLocale = 'ko' | 'en';

export type CalendarHoliday = {
	id: string;
	title: string;
	date: string;
	source: CalendarHolidaySource;
	countryCode?: string;
	readOnly: true;
	color: string;
};

export const holidayColor = '#dc2626';

export function calendarHolidayID(
	source: CalendarHolidaySource,
	sourceKey: string,
	externalID: string
): string {
	return `holiday:${source}:${sourceKey.trim()}:${externalID.trim()}`;
}

export function orderedCalendarHolidays(holidays: CalendarHoliday[]): CalendarHoliday[] {
	return [...holidays].sort((left, right) => {
		if (left.date !== right.date) return left.date < right.date ? -1 : 1;
		if (left.title !== right.title) return left.title < right.title ? -1 : 1;
		return left.id < right.id ? -1 : left.id > right.id ? 1 : 0;
	});
}
