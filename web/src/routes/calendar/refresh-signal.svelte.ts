import type { CalendarNavigationMessage } from './calendar-navigation-message';

export const calendarRefresh = $state({ ticks: 0 });
export const calendarVisibility = $state({ work: true });
export const calendarNavigation = $state({ dateKey: '' });

export const calendarChannelName = 'internkim-calendar';
export const calendarDateStorageKey = 'internkim.calendar.visibleDate';

export function bumpCalendarRefresh() {
	calendarRefresh.ticks += 1;
}

export function broadcastCalendarNavigation(date: Date) {
	window.localStorage.setItem(calendarDateStorageKey, date.toISOString());
	calendarNavigation.dateKey = dateKey(date);
	const channel = new BroadcastChannel(calendarChannelName);
	channel.postMessage({
		type: 'calendar-navigate',
		dateKey: dateKey(date)
	} satisfies CalendarNavigationMessage);
	channel.close();
}

function dateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}
