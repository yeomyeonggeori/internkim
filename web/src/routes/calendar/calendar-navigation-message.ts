export type CalendarNavigationMessage = {
	type: 'calendar-navigate';
	dateKey: string;
};

export type CalendarVisibleDateMessage = {
	type: 'calendar-visible-date';
	dateKey: string;
};

export type CalendarViewValue = 'day' | 'week' | 'month';

export type CalendarViewMessage = {
	type: 'calendar-view';
	view: CalendarViewValue;
};

export type CalendarEventsChangedMessage = {
	type: 'calendar-events-changed';
};

export type CalendarOpenSettingsMessage = {
	type: 'calendar-open-settings';
};

export type CalendarCreateEventMessage = {
	type: 'calendar-create-event';
};

export type CalendarRefreshMessage = {
	type: 'calendar-refresh';
};

const calendarDateKeyPattern = /^\d{4}-\d{2}-\d{2}$/;

export function isCalendarNavigationMessage(value: unknown): value is CalendarNavigationMessage {
	if (!value || typeof value !== 'object') return false;
	if (!('type' in value) || value.type !== 'calendar-navigate') return false;
	if (!('dateKey' in value) || typeof value.dateKey !== 'string') return false;
	return isCalendarDateKey(value.dateKey);
}

export function isCalendarVisibleDateMessage(value: unknown): value is CalendarVisibleDateMessage {
	if (!value || typeof value !== 'object') return false;
	if (!('type' in value) || value.type !== 'calendar-visible-date') return false;
	if (!('dateKey' in value) || typeof value.dateKey !== 'string') return false;
	return isCalendarDateKey(value.dateKey);
}

export function isCalendarViewMessage(value: unknown): value is CalendarViewMessage {
	if (!value || typeof value !== 'object') return false;
	if (!('type' in value) || value.type !== 'calendar-view') return false;
	if (!('view' in value) || typeof value.view !== 'string') return false;
	return isCalendarViewValue(value.view);
}

export function isCalendarEventsChangedMessage(value: unknown): value is CalendarEventsChangedMessage {
	if (!value || typeof value !== 'object') return false;
	return 'type' in value && value.type === 'calendar-events-changed';
}

export function isCalendarOpenSettingsMessage(value: unknown): value is CalendarOpenSettingsMessage {
	if (!value || typeof value !== 'object') return false;
	return 'type' in value && value.type === 'calendar-open-settings';
}

export function isCalendarCreateEventMessage(value: unknown): value is CalendarCreateEventMessage {
	if (!value || typeof value !== 'object') return false;
	return 'type' in value && value.type === 'calendar-create-event';
}

export function isCalendarRefreshMessage(value: unknown): value is CalendarRefreshMessage {
	if (!value || typeof value !== 'object') return false;
	return 'type' in value && value.type === 'calendar-refresh';
}

export function isCalendarViewValue(value: string): value is CalendarViewValue {
	return value === 'day' || value === 'week' || value === 'month';
}

function isCalendarDateKey(value: string): boolean {
	if (!calendarDateKeyPattern.test(value)) return false;
	const [yearText, monthText, dayText] = value.split('-');
	const year = Number(yearText);
	const month = Number(monthText);
	const day = Number(dayText);
	const date = new Date(year, month - 1, day);
	return date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day;
}
