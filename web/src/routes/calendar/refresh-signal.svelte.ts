import type {
	CalendarEventsChangedMessage,
	CalendarNavigationMessage,
	CalendarOpenSettingsMessage,
	CalendarRefreshMessage,
	CalendarViewMessage,
	CalendarViewValue,
	CalendarVisibleDateMessage
} from './calendar-navigation-message';
import { calendarDateStorageKey, calendarViewStorageKey } from './calendar-storage-keys';

export const calendarRefresh = $state({ ticks: 0 });
export const calendarNavigation = $state({ dateKey: '' });

export const calendarChannelName = 'internkim-calendar';

export function bumpCalendarRefresh() {
	calendarRefresh.ticks += 1;
}

export function broadcastCalendarNavigation(date: Date) {
	if (typeof window === 'undefined') return;
	window.localStorage.setItem(calendarDateStorageKey, date.toISOString());
	calendarNavigation.dateKey = dateKey(date);
	const channel = new BroadcastChannel(calendarChannelName);
	channel.postMessage({
		type: 'calendar-navigate',
		dateKey: dateKey(date)
	} satisfies CalendarNavigationMessage);
	channel.close();
}

export function broadcastCalendarVisibleDate(date: Date) {
	if (typeof window === 'undefined') return;
	window.localStorage.setItem(calendarDateStorageKey, date.toISOString());
	window.parent.postMessage(
		{
			type: 'calendar-visible-date',
			dateKey: dateKey(date)
		} satisfies CalendarVisibleDateMessage,
		window.location.origin
	);
}

export function broadcastCalendarView(view: CalendarViewValue) {
	if (typeof window === 'undefined') return;
	window.localStorage.setItem(calendarViewStorageKey, view);
	window.parent.postMessage(
		{
			type: 'calendar-view',
			view
		} satisfies CalendarViewMessage,
		window.location.origin
	);
}

export function broadcastCalendarEventsChanged() {
	if (typeof window === 'undefined') return;
	window.parent.postMessage(
		{
			type: 'calendar-events-changed'
		} satisfies CalendarEventsChangedMessage,
		window.location.origin
	);
}

export function requestCalendarSettingsOpen() {
	if (typeof window === 'undefined') return;
	window.parent.postMessage(
		{
			type: 'calendar-open-settings'
		} satisfies CalendarOpenSettingsMessage,
		window.location.origin
	);
}

export function requestCalendarRefresh() {
	if (typeof window === 'undefined') return;
	window.parent.postMessage(
		{
			type: 'calendar-refresh'
		} satisfies CalendarRefreshMessage,
		window.location.origin
	);
}

function dateKey(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}
