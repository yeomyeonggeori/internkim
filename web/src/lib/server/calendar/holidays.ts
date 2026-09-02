import {
	calendarHolidayID,
	holidayColor,
	orderedCalendarHolidays,
	type CalendarHoliday,
	type CalendarHolidayLocale
} from '$lib/calendar/holiday';
import {
	countryCodeOf,
	yearsBetween,
	type NationalHoliday,
	type NationalHolidayProvider
} from '$lib/server/national-holidays';

export type CompanyHolidayRecord = {
	id: string;
	title: string;
	date: string;
	recursAnnually: boolean;
};

// Nager answers a holiday in the country's own language and in English.
export function holidayTitleOf(
	countryCode: string,
	locale: CalendarHolidayLocale,
	holiday: NationalHoliday
): string {
	if (countryCode === 'KR' && locale === 'ko') return holiday.localName || holiday.name;
	return holiday.name || holiday.localName;
}

function isWithin(date: string, from: string, to: string): boolean {
	return date >= from && date < to;
}

function isRealDate(date: string): boolean {
	if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) return false;
	const parsed = new Date(`${date}T00:00:00Z`);
	return !Number.isNaN(parsed.valueOf()) && parsed.toISOString().slice(0, 10) === date;
}

function annualOccurrences(monthAndDay: string, from: string, to: string): string[] {
	const occurrences: string[] = [];
	for (let year = Number(from.slice(0, 4)); year <= Number(to.slice(0, 4)); year += 1) {
		const date = `${String(year).padStart(4, '0')}-${monthAndDay}`;
		if (isRealDate(date)) occurrences.push(date);
	}
	return occurrences;
}

function occurrencesOf(holiday: CompanyHolidayRecord, from: string, to: string): string[] {
	if (!isRealDate(holiday.date)) return [];
	if (!holiday.recursAnnually) return isWithin(holiday.date, from, to) ? [holiday.date] : [];
	return annualOccurrences(holiday.date.slice(5), from, to).filter((date) =>
		isWithin(date, from, to)
	);
}

export function companyCalendarHolidays(
	holidays: CompanyHolidayRecord[],
	from: string,
	to: string
): CalendarHoliday[] {
	return holidays.flatMap((holiday) =>
		occurrencesOf(holiday, from, to).map((date) => ({
			id: calendarHolidayID('company', holiday.id, date),
			title: holiday.title,
			date,
			source: 'company' as const,
			readOnly: true as const,
			color: holidayColor
		}))
	);
}

export async function nationalCalendarHolidays(
	provider: NationalHolidayProvider,
	country: string,
	locale: CalendarHolidayLocale,
	from: string,
	to: string
): Promise<CalendarHoliday[]> {
	const countryCode = countryCodeOf(country);
	const byYear = await Promise.all(
		yearsBetween(from, to).map((year) => provider.holidays(countryCode, year))
	);
	return byYear.flat().flatMap((holiday) => {
		if (!isWithin(holiday.date, from, to)) return [];
		const title = holidayTitleOf(countryCode, locale, holiday);
		return [
			{
				id: calendarHolidayID('holiday_api', `${countryCode}:${locale}`, `${holiday.date}:${title}`),
				title,
				date: holiday.date,
				source: 'holiday_api' as const,
				countryCode,
				readOnly: true as const,
				color: holidayColor
			}
		];
	});
}

export function mergedCalendarHolidays(
	national: CalendarHoliday[],
	company: CalendarHoliday[]
): CalendarHoliday[] {
	return orderedCalendarHolidays([...national, ...company]);
}
