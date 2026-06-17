import { ViewType } from '@dayflow/svelte';
import { calendarViewType } from './calendar-embed-view-helpers';
import { loadSavedCalendarDate, loadSavedCalendarView } from './calendar-storage';

export function initialCalendarDate(isBrowser: boolean, searchParams: URLSearchParams): Date {
	if (!isBrowser) return loadSavedCalendarDate(isBrowser);
	const dateValue = searchParams.get('date') ?? '';
	if (!dateValue) return loadSavedCalendarDate(isBrowser);
	const parsedDate = new Date(`${dateValue}T00:00:00`);
	if (Number.isNaN(parsedDate.getTime())) return loadSavedCalendarDate(isBrowser);
	return parsedDate;
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
