import { ViewType } from '@dayflow/svelte';
import { calendarViewType } from './calendar-embed-view-helpers';
import { loadSavedCalendarView } from './calendar-storage';

export function initialCalendarDate(isBrowser: boolean, searchParams: URLSearchParams, today = new Date()): Date {
	if (!isBrowser) return today;
	const dateValue = searchParams.get('date') ?? '';
	if (!dateValue) return today;
	const parsedDate = new Date(`${dateValue}T00:00:00`);
	return Number.isNaN(parsedDate.getTime()) ? today : parsedDate;
}

export function initialCalendarEventID(isBrowser: boolean, searchParams: URLSearchParams): string {
	if (!isBrowser) return '';
	return searchParams.get('event') ?? '';
}

export function initialCalendarView(isBrowser: boolean): ViewType {
	return calendarViewType(loadSavedCalendarView(isBrowser));
}

export function calendarSearchParams(isBrowser: boolean): URLSearchParams {
	if (!isBrowser) return new URLSearchParams();
	return new URLSearchParams(window.location.search);
}
