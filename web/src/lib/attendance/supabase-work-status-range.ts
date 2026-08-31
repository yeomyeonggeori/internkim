import { companyInstantOf } from '$lib/company-time';

export type SupabaseWorkStatusTimeRange = {
	from: string;
	until: string;
};

export function supabaseWorkStatusTimeRange(
	days: readonly string[],
	timeZone: string
): SupabaseWorkStatusTimeRange {
	const firstDay = days[0];
	const lastDay = days.at(-1);
	if (!firstDay || !lastDay) throw new Error('work status time range requires at least one day');
	return {
		from: companyTimeInstant(firstDay, '00:00', timeZone),
		until: companyTimeInstant(shiftedDay(lastDay, 1), '00:00', timeZone)
	};
}

export function companyMonthTimeRange(
	month: string,
	timeZone: string
): SupabaseWorkStatusTimeRange {
	const [year, monthNumber] = month.split('-').map(Number);
	if (!Number.isInteger(year) || !Number.isInteger(monthNumber) || monthNumber < 1 || monthNumber > 12) {
		throw new Error(`invalid company month: ${month}`);
	}
	const firstDayOfNextMonth = new Date(Date.UTC(year, monthNumber, 1)).toISOString().slice(0, 10);
	return {
		from: companyTimeInstant(`${month}-01`, '00:00', timeZone),
		until: companyTimeInstant(firstDayOfNextMonth, '00:00', timeZone)
	};
}

export function shiftedDay(day: string, days: number): string {
	const moved = new Date(`${day}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + days);
	return moved.toISOString().slice(0, 10);
}

export function companyTimeInstant(day: string, time: string, timeZone: string): string {
	if (!/^\d{2}:\d{2}$/.test(time)) throw new Error(`invalid company time: ${time}`);
	const [hour, minute] = time.split(':').map(Number);
	if (hour > 23 || minute > 59) throw new Error(`invalid company time: ${time}`);
	const instant = companyInstantOf(day, time, timeZone);
	if (!instant) throw new Error(`work status date does not exist in company timezone: ${day} ${timeZone}`);
	return instant;
}
