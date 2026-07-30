import {
	isCalendarViewValue,
	type CalendarViewValue
} from '../calendar-navigation-message';
import { calendarViewStorageKey } from '../calendar-storage-keys';

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
