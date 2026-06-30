import type { CalendarParticipant, CalendarParticipantInput } from './calendar-participants';

export type CalendarEvent = {
	id: string;
	uid: string;
	title: string;
	description: string;
	location: string;
	startISO: string;
	endISO: string;
	timeZone: string;
	isAllDay: boolean;
	color: string;
	participants?: CalendarParticipant[];
	createdByEmail: string;
	createdByName: string;
	createdByImage?: string;
	updatedByEmail?: string;
	updatedByName?: string;
	updatedByImage?: string;
	updatedByAt?: string;
	updatedAt: string;
};

export type CalendarEventPayload = {
	eventID: string;
	title: string;
	description: string;
	location: string;
	startISO: string;
	endISO: string;
	timeZone: string;
	isAllDay: boolean;
	color: string;
	participants: CalendarParticipantInput[];
};

type CalendarEventsResponse = {
	events: CalendarEvent[];
};

export async function fetchCalendarEvents(startDate: Date, endDate: Date, errorFallback: string): Promise<CalendarEvent[]> {
	const query = new URLSearchParams({
		startISO: startDate.toISOString(),
		endISO: endDate.toISOString()
	});
	const response = await fetch(`/calendar/api/events?${query}`, { credentials: 'include' });
	if (!response.ok) throw new Error(await responseErrorMessage(response, errorFallback));
	const document = (await response.json()) as CalendarEventsResponse;
	return document.events;
}

export async function writeCalendarEvent(
	path: string,
	method: 'POST' | 'PUT',
	payload: CalendarEventPayload,
	errorFallback: string
): Promise<CalendarEvent> {
	const response = await fetch(path, {
		method,
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	});
	if (!response.ok) throw new Error(await responseErrorMessage(response, errorFallback));
	return (await response.json()) as CalendarEvent;
}

export async function deletePersistedCalendarEvent(eventID: string, errorFallback: string): Promise<void> {
	const response = await fetch(`/calendar/api/events/${encodeURIComponent(eventID)}`, {
		method: 'DELETE',
		credentials: 'include'
	});
	if (!response.ok) throw new Error(await responseErrorMessage(response, errorFallback));
}

export async function responseErrorMessage(response: Response, fallback: string): Promise<string> {
	const message = (await response.text()).trim();
	if (!message || message.startsWith('<!doctype html>') || message.startsWith('<html')) return fallback;
	return message;
}
