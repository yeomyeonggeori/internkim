import {
	isCalendarViewValue,
	type CalendarViewValue
} from '../calendar-navigation-message';
import { calendarDateStorageKey, calendarViewStorageKey } from '../calendar-storage-keys';

export function loadSavedCalendarDate(isBrowser: boolean): Date {
	if (!isBrowser) return new Date();
	try {
		const saved = window.localStorage.getItem(calendarDateStorageKey);
		if (!saved) return new Date();
		const parsed = new Date(saved);
		return Number.isNaN(parsed.getTime()) ? new Date() : parsed;
	} catch {
		return new Date();
	}
}

export function loadSavedCalendarView(isBrowser: boolean, storage: Storage | null = browserStorage(isBrowser)): CalendarViewValue {
	if (!storage) return 'month';
	try {
		const saved = storage.getItem(calendarViewStorageKey);
		return saved && isCalendarViewValue(saved) ? saved : 'month';
	} catch {
		return 'month';
	}
}

function browserStorage(isBrowser: boolean): Storage | null {
	if (!isBrowser) return null;
	return window.localStorage;
}
