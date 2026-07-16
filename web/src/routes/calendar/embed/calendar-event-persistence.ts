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

const calendarTargetUnavailableErrorCode = 'calendar_target_unavailable';

export type CalendarPersistenceErrorCode = typeof calendarTargetUnavailableErrorCode | 'unknown';

export class CalendarPersistenceError extends Error {
	constructor(
		readonly code: CalendarPersistenceErrorCode,
		message: string
	) {
		super(message);
		this.name = 'CalendarPersistenceError';
	}
}

export async function fetchCalendarEvents(startDate: Date, endDate: Date, errorFallback: string): Promise<CalendarEvent[]> {
	const query = new URLSearchParams({
		startISO: startDate.toISOString(),
		endISO: endDate.toISOString()
	});
	const response = await fetch(`/calendar/api/events?${query}`, { credentials: 'include' });
	if (!response.ok) throw new Error(await responseErrorMessage(response, errorFallback));
	const document = (await response.json()) as CalendarEventsResponse;
	return document.events ?? [];
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
	if (!response.ok) throw await calendarPersistenceErrorFromResponse(response, errorFallback);
	return (await response.json()) as CalendarEvent;
}

export async function deletePersistedCalendarEvent(eventID: string, errorFallback: string): Promise<void> {
	const response = await fetch(`/calendar/api/events/${encodeURIComponent(eventID)}`, {
		method: 'DELETE',
		credentials: 'include'
	});
	if (!response.ok) throw await calendarPersistenceErrorFromResponse(response, errorFallback);
}

export function deletePersistedCalendarEventOnPageHide(eventID: string): void {
	void fetch(`/calendar/api/events/${encodeURIComponent(eventID)}`, {
		method: 'DELETE',
		credentials: 'include',
		keepalive: true
	}).catch((error: unknown) => {
		console.warn('calendar delete keepalive request failed', { error });
	});
}

export async function responseErrorMessage(response: Response, fallback: string): Promise<string> {
	return responseBodyErrorMessage((await response.text()).trim(), fallback);
}

export async function calendarPersistenceErrorFromResponse(
	response: Response,
	fallback: string
): Promise<CalendarPersistenceError> {
	const responseBody = (await response.text()).trim();
	const isJSONResponse = response.headers.get('Content-Type')?.toLowerCase().includes('application/json') === true;
	const code = decodeCalendarPersistenceErrorCode(responseBody);
	const message = isJSONResponse || responseBody.startsWith('{')
		? fallback
		: responseBodyErrorMessage(responseBody, fallback);
	return new CalendarPersistenceError(code, message);
}

function decodeCalendarPersistenceErrorCode(responseBody: string): CalendarPersistenceErrorCode {
	if (!responseBody.startsWith('{')) return 'unknown';
	let document: unknown;
	try {
		document = JSON.parse(responseBody);
	} catch (error: unknown) {
		if (!(error instanceof SyntaxError)) throw error;
		return 'unknown';
	}
	if (!document || typeof document !== 'object' || !('code' in document)) return 'unknown';
	return document.code === calendarTargetUnavailableErrorCode ? calendarTargetUnavailableErrorCode : 'unknown';
}

function responseBodyErrorMessage(message: string, fallback: string): string {
	if (!message || message.startsWith('<!doctype html>') || message.startsWith('<html')) return fallback;
	return message;
}
