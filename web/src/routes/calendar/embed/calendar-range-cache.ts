import type { CalendarEvent } from './calendar-event-persistence';
import type { CalendarHoliday } from './calendar-holiday-persistence';

const storageKey = 'internkim.calendar.ranges.v1';
const maximumRanges = 4;
const maximumAgeMilliseconds = 24 * 60 * 60 * 1000;

export type StoredCalendarRange = {
	scope: string;
	rangeKey: string;
	savedAt: number;
	events: CalendarEvent[];
	holidays: CalendarHoliday[];
};

export function readStoredCalendarRange(scope: string, rangeKey: string, now = Date.now()): StoredCalendarRange | null {
	if (!scope) return null;
	const stored = storedRanges().find((range) => range.scope === scope && range.rangeKey === rangeKey);
	if (!stored) return null;
	const age = now - stored.savedAt;
	return age >= 0 && age <= maximumAgeMilliseconds ? stored : null;
}

export function storeCalendarRange(
	scope: string,
	rangeKey: string,
	contents: Partial<Pick<StoredCalendarRange, 'events' | 'holidays'>>,
	savedAt = Date.now()
): void {
	if (!scope || typeof window === 'undefined') return;
	const ranges = storedRanges();
	const previous = ranges.find((range) => range.scope === scope && range.rangeKey === rangeKey);
	const next: StoredCalendarRange = {
		scope,
		rangeKey,
		savedAt,
		events: contents.events ?? previous?.events ?? [],
		holidays: contents.holidays ?? previous?.holidays ?? []
	};
	const kept = ranges.filter((range) => range !== previous && range.scope === scope);
	try {
		window.localStorage.setItem(storageKey, JSON.stringify([next, ...kept].slice(0, maximumRanges)));
	} catch {
		return;
	}
}

export function forgetStoredCalendarRanges(): void {
	if (typeof window === 'undefined') return;
	try {
		window.localStorage.removeItem(storageKey);
	} catch {
		return;
	}
}

function storedRanges(): StoredCalendarRange[] {
	if (typeof window === 'undefined') return [];
	try {
		const parsed: unknown = JSON.parse(window.localStorage.getItem(storageKey) ?? '[]');
		return Array.isArray(parsed) ? parsed.filter(isStoredCalendarRange) : [];
	} catch {
		return [];
	}
}

function isStoredCalendarRange(value: unknown): value is StoredCalendarRange {
	if (typeof value !== 'object' || value === null) return false;
	const range: Partial<StoredCalendarRange> = value;
	return typeof range.scope === 'string' && typeof range.rangeKey === 'string' &&
		Number.isFinite(range.savedAt) && Array.isArray(range.events) && Array.isArray(range.holidays);
}
