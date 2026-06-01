const calendarDateStorageKey = 'internkim.calendar.visibleDate';

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

export function saveCalendarDate(isBrowser: boolean, date: Date): void {
	if (!isBrowser) return;
	try {
		window.localStorage.setItem(calendarDateStorageKey, date.toISOString());
	} catch {
		return;
	}
}
