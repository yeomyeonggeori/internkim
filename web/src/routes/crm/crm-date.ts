import { browserTimeZone, utcToLocalDate } from './crm-mappers';

export type CRMReportPeriodBounds = {
	today: string;
	next90DaysEnd: string;
	quarterStart: string;
	quarterEnd: string;
};

export function currentCRMDate(now = new Date(), timeZone = browserTimeZone()): string {
	return utcToLocalDate(now.toISOString(), timeZone);
}

export function shiftCRMDate(date: string, days: number): string {
	const [year, month, day] = date.split('-').map(Number);
	if (!year || !month || !day) return '';
	return formatUTCDate(new Date(Date.UTC(year, month - 1, day + days)));
}

export function buildCRMReportPeriodBounds(
	now = new Date(),
	timeZone = browserTimeZone()
): CRMReportPeriodBounds {
	const today = currentCRMDate(now, timeZone);
	const [year, month] = today.split('-').map(Number);
	const quarterMonth = Math.floor((month - 1) / 3) * 3;
	return {
		today,
		next90DaysEnd: shiftCRMDate(today, 90),
		quarterStart: formatUTCDate(new Date(Date.UTC(year, quarterMonth, 1))),
		quarterEnd: formatUTCDate(new Date(Date.UTC(year, quarterMonth + 3, 0)))
	};
}

function formatUTCDate(date: Date): string {
	return date.toISOString().slice(0, 10);
}
