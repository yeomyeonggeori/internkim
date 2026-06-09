export type CalendarNavigationMessage = {
	type: 'calendar-navigate';
	dateKey: string;
};

const calendarDateKeyPattern = /^\d{4}-\d{2}-\d{2}$/;

export function isCalendarNavigationMessage(value: unknown): value is CalendarNavigationMessage {
	if (!value || typeof value !== 'object') return false;
	if (!('type' in value) || value.type !== 'calendar-navigate') return false;
	if (!('dateKey' in value) || typeof value.dateKey !== 'string') return false;
	return isCalendarDateKey(value.dateKey);
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
