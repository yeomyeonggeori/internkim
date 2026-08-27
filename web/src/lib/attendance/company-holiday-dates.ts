import type { CompanyHoliday } from '../../routes/admin/admin-types';

export function companyHolidayDatesBetween(
	holidays: CompanyHoliday[],
	startDate: string,
	endDate: string
): string[] {
	const found = new Set<string>();
	for (const holiday of holidays) {
		for (const date of holidayOccurrences(holiday, startDate, endDate)) {
			if (date >= startDate && date < endDate) found.add(date);
		}
	}
	return [...found].sort();
}

export function companyHolidayDatesInMonth(
	holidays: CompanyHoliday[],
	month: string
): string[] {
	return companyHolidayDatesBetween(holidays, `${month}-01`, monthAfter(month));
}

export function monthAfter(month: string): string {
	const [year, monthNumber] = month.split('-').map(Number);
	return monthNumber === 12
		? `${year + 1}-01-01`
		: `${year}-${String(monthNumber + 1).padStart(2, '0')}-01`;
}

function holidayOccurrences(
	holiday: CompanyHoliday,
	startDate: string,
	endDate: string
): string[] {
	if (!holiday.recursAnnually) return [holiday.date];
	const [, month, day] = holiday.date.split('-').map(Number);
	const firstYear = Number(startDate.slice(0, 4));
	const lastYear = Number(endDate.slice(0, 4));
	const occurrences: string[] = [];
	for (let year = firstYear; year <= lastYear; year += 1) {
		const date = calendarDate(year, month, day);
		if (date) occurrences.push(date);
	}
	return occurrences;
}

function calendarDate(year: number, month: number, day: number): string | null {
	const date = new Date(Date.UTC(year, month - 1, day));
	if (date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) return null;
	return date.toISOString().slice(0, 10);
}
