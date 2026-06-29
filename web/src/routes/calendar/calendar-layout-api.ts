import type {
	CalendarAccountStatusResponse,
	CalendarEvent,
	CalendarEventsResponse,
	CalendarSyncResponse
} from './calendar-layout-types';

export async function fetchCalendarSyncInformation(): Promise<CalendarSyncResponse | null> {
	const response = await fetch('/calendar/api/sync', { credentials: 'include' });
	if (!response.ok) return null;
	return (await response.json()) as CalendarSyncResponse;
}

export async function fetchCalendarAccountStatus(): Promise<CalendarAccountStatusResponse> {
	const response = await fetch('/calendar/api/account-status', { credentials: 'include' });
	if (!response.ok) throw new Error('Calendar account status request failed');
	return (await response.json()) as CalendarAccountStatusResponse;
}

export async function fetchCalendarEventsForMonth(monthDate: Date): Promise<CalendarEvent[]> {
	const startDate = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1);
	const endDate = new Date(monthDate.getFullYear(), monthDate.getMonth() + 1, 1);
	const query = new URLSearchParams({
		startISO: startDate.toISOString(),
		endISO: endDate.toISOString()
	});
	const response = await fetch(`/calendar/api/events?${query}`, { credentials: 'include' });
	if (!response.ok) return [];
	const payload = (await response.json()) as CalendarEventsResponse;
	return payload.events ?? [];
}

export async function rotateCalendarSubscriptionURL(errorMessage: string): Promise<CalendarSyncResponse> {
	const response = await fetch('/calendar/api/ics-token', {
		method: 'POST',
		credentials: 'include'
	});
	if (!response.ok) throw new Error(errorMessage);
	return (await response.json()) as CalendarSyncResponse;
}

export async function uploadGoogleOAuthClient(file: File, errorMessage: string): Promise<void> {
	const formData = new FormData();
	formData.append('client', file, file.name);
	const response = await fetch('/calendar/api/google-oauth-client', {
		method: 'POST',
		credentials: 'include',
		body: formData
	});
	if (!response.ok) {
		const responseText = (await response.text()).trim();
		throw new Error(responseText || errorMessage);
	}
}
