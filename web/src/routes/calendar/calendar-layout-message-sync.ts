import { calendarDateFromKey } from './calendar-layout-date';
import {
	isCalendarEventsChangedMessage,
	isCalendarOpenSettingsMessage,
	isCalendarRefreshMessage,
	isCalendarVisibleDateMessage
} from './calendar-navigation-message';
import { calendarDateStorageKey } from './calendar-storage-keys';

export type CalendarLayoutMessageSyncOptions = {
	selectedDateKey: () => string;
	applyVisibleDate: (date: Date) => void;
	openSettings: () => void;
	refreshCalendar: () => void;
};

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
		if (isCalendarOpenSettingsMessage(message)) {
			options.openSettings();
			return;
		}
		if (isCalendarRefreshMessage(message)) {
			options.refreshCalendar();
			return;
		}
		if (isCalendarEventsChangedMessage(message)) return;
	};

	const handleCalendarStorageChange = (event: StorageEvent) => {
		if (event.key === calendarDateStorageKey && event.newValue) {
			const visibleDate = new Date(event.newValue);
			if (!Number.isNaN(visibleDate.getTime())) options.applyVisibleDate(visibleDate);
		}
	};

	window.addEventListener('message', handleCalendarFrameMessage);
	window.addEventListener('storage', handleCalendarStorageChange);
	return () => {
		window.removeEventListener('message', handleCalendarFrameMessage);
		window.removeEventListener('storage', handleCalendarStorageChange);
	};
}
