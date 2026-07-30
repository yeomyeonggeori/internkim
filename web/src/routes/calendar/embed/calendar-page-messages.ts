import { calendarDateStorageKey } from '../calendar-storage-keys';
import { isCalendarCreateEventMessage, isCalendarNavigationMessage } from '../calendar-navigation-message';
import { dateKey } from './calendar-draft-popover-state';

type CalendarPageMessageContext = {
	getCurrentOrigin: () => string;
	navigateToDateKey: (dateKey: string) => void;
	createQuickEvent: () => void;
};

export type CalendarPageMessageActions = {
	handleCalendarChannelMessage: (event: MessageEvent<unknown>) => void;
	handleCalendarStorageMessage: (event: StorageEvent) => void;
	handleCalendarWindowMessage: (event: MessageEvent<unknown>) => void;
};

export function createCalendarPageMessageActions(context: CalendarPageMessageContext): CalendarPageMessageActions {
	function handleCalendarChannelMessage(event: MessageEvent<unknown>): void {
		if (isCalendarNavigationMessage(event.data)) {
			context.navigateToDateKey(event.data.dateKey);
		}
	}

	function handleCalendarWindowMessage(event: MessageEvent<unknown>): void {
		if (event.origin !== context.getCurrentOrigin()) return;
		if (isCalendarCreateEventMessage(event.data)) {
			context.createQuickEvent();
			return;
		}
		if (!isCalendarNavigationMessage(event.data)) return;
		context.navigateToDateKey(event.data.dateKey);
	}

	function handleCalendarStorageMessage(event: StorageEvent): void {
		if (event.key !== calendarDateStorageKey || !event.newValue) return;
		const date = new Date(event.newValue);
		if (Number.isNaN(date.getTime())) return;
		context.navigateToDateKey(dateKey(date));
	}

	return {
		handleCalendarChannelMessage,
		handleCalendarStorageMessage,
		handleCalendarWindowMessage
	};
}
