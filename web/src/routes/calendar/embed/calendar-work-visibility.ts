export type CalendarVisibilityMessage = {
	type: 'calendar-visibility';
	work: boolean;
};

const calendarWorkVisibilityStorageKey = 'internkim.calendar.workVisible';

export function loadSavedWorkCalendarVisibility(storage: Storage): boolean {
	return storage.getItem(calendarWorkVisibilityStorageKey) !== 'false';
}

export function isCalendarVisibilityMessage(value: unknown): value is CalendarVisibilityMessage {
	if (typeof value !== 'object' || value === null) return false;
	return 'type' in value && 'work' in value && value.type === 'calendar-visibility' && typeof value.work === 'boolean';
}
