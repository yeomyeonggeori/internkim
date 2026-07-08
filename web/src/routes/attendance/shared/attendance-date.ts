export function todayDateInTimeZone(timeZone?: string, date: Date = new Date()): string {
	const parts = datePartsInTimeZone(timeZone, date);
	return `${parts.year}-${parts.month}-${parts.day}`;
}

export function currentMonthInTimeZone(timeZone?: string, date: Date = new Date()): string {
	const parts = datePartsInTimeZone(timeZone, date);
	return `${parts.year}-${parts.month}`;
}

export function timeInTimeZone(timeZone?: string, date: Date = new Date()): string {
	const parts = timePartsInTimeZone(timeZone, date);
	return `${parts.hour}:${parts.minute}`;
}

export function timeZoneDisplayLabel(timeZone?: string, date: Date = new Date()): string {
	const zone = normalizeTimeZone(timeZone);
	const offset = new Intl.DateTimeFormat('en-US', { timeZone: zone, timeZoneName: 'shortOffset' })
		.formatToParts(date)
		.find((part) => part.type === 'timeZoneName')?.value;
	const city = zone.split('/').pop()?.replaceAll('_', ' ') ?? zone;
	return offset ? `${city} (${offset})` : city;
}

export function utcDateKey(date: Date): string {
	return [
		String(date.getUTCFullYear()).padStart(4, '0'),
		String(date.getUTCMonth() + 1).padStart(2, '0'),
		String(date.getUTCDate()).padStart(2, '0'),
	].join('-');
}

export function eachDayOfMonth(month: string): string[] {
	const [year, monthIndex] = month.split('-').map(Number);
	if (!year || !monthIndex) return [];
	const days: string[] = [];
	const date = new Date(Date.UTC(year, monthIndex - 1, 1));
	while (date.getUTCMonth() === monthIndex - 1) {
		days.push(utcDateKey(date));
		date.setUTCDate(date.getUTCDate() + 1);
	}
	return days;
}

export function addDays(date: string, count: number): string {
	const nextDate = new Date(`${date}T00:00:00Z`);
	if (Number.isNaN(nextDate.getTime())) return date;
	nextDate.setUTCDate(nextDate.getUTCDate() + count);
	return utcDateKey(nextDate);
}

export function eachDayOfWeek(date: string): string[] {
	const startDate = isoWeekStart(date);
	return Array.from({ length: 7 }, (_, index) => addDays(startDate, index));
}

export function isWeekday(date: string): boolean {
	const day = new Date(`${date}T00:00:00Z`).getUTCDay();
	return day !== 0 && day !== 6;
}

export function isWeekend(date: string): boolean {
	return !isWeekday(date);
}

export function isoWeekStart(date: string): string {
	const d = new Date(`${date}T00:00:00Z`);
	const dow = d.getUTCDay();
	const diff = dow === 0 ? -6 : 1 - dow;
	d.setUTCDate(d.getUTCDate() + diff);
	return utcDateKey(d);
}

function datePartsInTimeZone(timeZone: string | undefined, date: Date): { year: string; month: string; day: string } {
	const parts = new Intl.DateTimeFormat('en-US', {
		timeZone: normalizeTimeZone(timeZone),
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
	}).formatToParts(date);
	return {
		year: partValue(parts, 'year'),
		month: partValue(parts, 'month'),
		day: partValue(parts, 'day'),
	};
}

function timePartsInTimeZone(timeZone: string | undefined, date: Date): { hour: string; minute: string } {
	const parts = new Intl.DateTimeFormat('en-US', {
		timeZone: normalizeTimeZone(timeZone),
		hour: '2-digit',
		minute: '2-digit',
		hour12: false,
	}).formatToParts(date);
	return {
		hour: partValue(parts, 'hour'),
		minute: partValue(parts, 'minute'),
	};
}

function normalizeTimeZone(timeZone: string | undefined): string {
	const trimmedTimeZone = timeZone?.trim();
	if (!trimmedTimeZone || trimmedTimeZone === 'Local') return browserTimeZone();
	if (isValidTimeZone(trimmedTimeZone)) return trimmedTimeZone;
	return browserTimeZone();
}

function browserTimeZone(): string {
	return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
}

function isValidTimeZone(timeZone: string): boolean {
	try {
		new Intl.DateTimeFormat('en-US', { timeZone });
		return true;
	} catch (error) {
		if (error instanceof RangeError) return false;
		throw error;
	}
}

function partValue(parts: Intl.DateTimeFormatPart[], type: Intl.DateTimeFormatPartTypes): string {
	return parts.find((part) => part.type === type)?.value ?? '';
}
