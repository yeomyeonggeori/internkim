import { calendarDateFromKey, calendarDateKey } from './calendar-layout-date';
import {
	isCalendarEventsChangedMessage,
	isCalendarViewMessage,
	isCalendarViewValue,
	isCalendarVisibleDateMessage,
	type CalendarViewValue
} from './calendar-navigation-message';
import { calendarDateStorageKey, calendarViewStorageKey } from './calendar-storage-keys';
import { calendarChannelName } from './refresh-signal.svelte';

const calendarWorkVisibilityStorageKey = 'internkim.calendar.workVisible';

export type CalendarLayoutStoredState = {
	visibleDate: Date | null;
	view: CalendarViewValue | null;
	isWorkVisible: boolean | null;
};

export type CalendarLayoutMessageSyncOptions = {
	selectedDateKey: () => string;
	applyVisibleDate: (date: Date) => void;
	applyCalendarView: (view: CalendarViewValue) => void;
	reloadMiniMonthEvents: () => void;
};

export function loadCalendarLayoutStoredState(): CalendarLayoutStoredState {
	const visibleDate = storedVisibleDate();
	const view = storedCalendarView();
	const storedVisibility = window.localStorage.getItem(calendarWorkVisibilityStorageKey);
	return {
		visibleDate,
		view,
		isWorkVisible: storedVisibility === null ? null : storedVisibility === 'true'
	};
}

export function saveAndBroadcastCalendarWorkVisibility(isVisible: boolean): void {
	window.localStorage.setItem(calendarWorkVisibilityStorageKey, String(isVisible));
	const channel = new BroadcastChannel(calendarChannelName);
	channel.postMessage({ type: 'calendar-visibility', work: isVisible });
	channel.close();
}

export function installCalendarLayoutMessageSync(options: CalendarLayoutMessageSyncOptions): () => void {
	const handleCalendarFrameMessage = (event: MessageEvent<unknown>) => {
		if (event.origin !== window.location.origin) return;
		const message = event.data;
		if (isCalendarVisibleDateMessage(message)) {
			if (message.dateKey === options.selectedDateKey()) return;
			const visibleDate = calendarDateFromKey(message.dateKey);
			window.localStorage.setItem(calendarDateStorageKey, visibleDate.toISOString());
			options.applyVisibleDate(visibleDate);
			return;
		}
		if (isCalendarViewMessage(message)) {
			window.localStorage.setItem(calendarViewStorageKey, message.view);
			options.applyCalendarView(message.view);
			return;
		}
		if (isCalendarEventsChangedMessage(message)) options.reloadMiniMonthEvents();
	};

	const handleCalendarStorageChange = (event: StorageEvent) => {
		if (event.key === calendarDateStorageKey && event.newValue) {
			const visibleDate = new Date(event.newValue);
			if (!Number.isNaN(visibleDate.getTime())) options.applyVisibleDate(visibleDate);
		}
		if (event.key === calendarViewStorageKey && event.newValue && isCalendarViewValue(event.newValue)) {
			options.applyCalendarView(event.newValue);
		}
	};

	window.addEventListener('message', handleCalendarFrameMessage);
	window.addEventListener('storage', handleCalendarStorageChange);
	return () => {
		window.removeEventListener('message', handleCalendarFrameMessage);
		window.removeEventListener('storage', handleCalendarStorageChange);
	};
}

function storedVisibleDate(): Date | null {
	const savedVisibleDate = window.localStorage.getItem(calendarDateStorageKey);
	if (!savedVisibleDate) return null;
	const visibleDate = new Date(savedVisibleDate);
	return Number.isNaN(visibleDate.getTime()) ? null : visibleDate;
}

function storedCalendarView(): CalendarViewValue | null {
	const savedCalendarView = window.localStorage.getItem(calendarViewStorageKey);
	if (!savedCalendarView || !isCalendarViewValue(savedCalendarView)) return null;
	return savedCalendarView;
}

export function initialSelectedDateKey(today: Date, visibleDate: Date | null): string {
	return calendarDateKey(visibleDate ?? today);
}
