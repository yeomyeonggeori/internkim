import { companyDateOf, companyInstantOf, companyTimeOf, isValidTimeZone } from '../../../lib/company-time';

export function todayDateInTimeZone(timeZone?: string, date: Date = new Date()): string {
	return companyDateOf(date, normalizeTimeZone(timeZone));
}

export function currentMonthInTimeZone(timeZone?: string, date: Date = new Date()): string {
	return todayDateInTimeZone(timeZone, date).slice(0, 7);
}

export function timeInTimeZone(timeZone?: string, date: Date = new Date()): string {
	return companyTimeOf(date, normalizeTimeZone(timeZone));
}

export function isFutureAttendanceLocalTime(
	localDate: string,
	localTime: string,
	timeZone?: string,
	now: Date = new Date()
): boolean {
	const currentLocalDate = todayDateInTimeZone(timeZone, now);
	if (localDate !== currentLocalDate) return localDate > currentLocalDate;
	return localTime > timeInTimeZone(timeZone, now);
}

export function fallbackFutureAttendanceLocalTime(
	localDate: string,
	localTime: string,
	timeZone?: string,
	now: Date = new Date()
): string {
	if (localDate !== todayDateInTimeZone(timeZone, now)) return localTime;
	if (!isFutureAttendanceLocalTime(localDate, localTime, timeZone, now)) return localTime;
	return timeInTimeZone(timeZone, now);
}

const localDatePattern = /^\d{4}-\d{2}-\d{2}$/;
const localTimePattern = /^\d{2}:\d{2}$/;

export function attendanceInstantOfLocalTime(
	localDate: string,
	localTime: string,
	timeZone?: string
): Date {
	if (!localDatePattern.test(localDate) || !localTimePattern.test(localTime)) {
		return new Date(Number.NaN);
	}
	const instant = companyInstantOf(localDate, localTime, normalizeTimeZone(timeZone));
	return instant ? new Date(instant) : new Date(Number.NaN);
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

const normalizedTimeZoneByInput = new Map<string, string>();

function normalizeTimeZone(timeZone: string | undefined): string {
	const trimmedTimeZone = timeZone?.trim() ?? '';
	const cachedTimeZone = normalizedTimeZoneByInput.get(trimmedTimeZone);
	if (cachedTimeZone) return cachedTimeZone;
	const normalizedTimeZone = resolveTimeZone(trimmedTimeZone);
	normalizedTimeZoneByInput.set(trimmedTimeZone, normalizedTimeZone);
	return normalizedTimeZone;
}

function resolveTimeZone(trimmedTimeZone: string): string {
	if (!trimmedTimeZone || trimmedTimeZone === 'Local') return browserTimeZone();
	if (isValidTimeZone(trimmedTimeZone)) return trimmedTimeZone;
	return browserTimeZone();
}

function browserTimeZone(): string {
	return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
}
