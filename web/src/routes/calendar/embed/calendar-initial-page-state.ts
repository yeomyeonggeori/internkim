import { ViewType } from '../calendar-view-type';
import { calendarViewType } from './calendar-embed-view-helpers';
import { loadSavedCalendarView } from './calendar-storage';

export function initialCalendarDate(searchParams: URLSearchParams, today = new Date()): Date {
	const dateValue = searchParams.get('date') ?? '';
	if (!dateValue) return today;
	const parsedDate = new Date(`${dateValue}T00:00:00`);
	return Number.isNaN(parsedDate.getTime()) ? today : parsedDate;
}

export function initialCalendarEventID(searchParams: URLSearchParams): string {
	return searchParams.get('event') ?? '';
}

export function initialCalendarView(isBrowser: boolean): ViewType {
	return calendarViewType(loadSavedCalendarView(isBrowser));
}
