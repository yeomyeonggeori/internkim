import { getContext, setContext } from 'svelte';

const key = Symbol('calendar-settings');

export function setCalendarSettings(open: () => void): void {
	setContext(key, open);
}

export function calendarSettings(): (() => void) | undefined {
	return getContext(key);
}
