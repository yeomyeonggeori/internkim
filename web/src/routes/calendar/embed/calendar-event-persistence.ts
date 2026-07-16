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
	expectedUpdatedAt?: string;
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
const calendarEventVersionConflictErrorCode = 'calendar_event_version_conflict';

export type CalendarPersistenceErrorCode =
	| typeof calendarTargetUnavailableErrorCode
	| typeof calendarEventVersionConflictErrorCode
	| 'unknown';

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

export async function deletePersistedCalendarEvent(
	eventID: string,
	expectedUpdatedAt: string | undefined,
	errorFallback: string
): Promise<void> {
	const response = await fetch(`/calendar/api/events/${encodeURIComponent(eventID)}`, {
		method: 'DELETE',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ expectedUpdatedAt })
	});
	if (!response.ok) throw await calendarPersistenceErrorFromResponse(response, errorFallback);
}

export function deletePersistedCalendarEventOnPageHide(
	eventID: string,
	expectedUpdatedAt: string | undefined
): void {
	void fetch(`/calendar/api/events/${encodeURIComponent(eventID)}`, {
		method: 'DELETE',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ expectedUpdatedAt }),
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
	const code = decodeCalendarPersistenceErrorCode(responseBody);
	return new CalendarPersistenceError(code, fallback);
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
	if (document.code === calendarTargetUnavailableErrorCode) return calendarTargetUnavailableErrorCode;
	if (document.code === calendarEventVersionConflictErrorCode) return calendarEventVersionConflictErrorCode;
	return 'unknown';
}

function responseBodyErrorMessage(message: string, fallback: string): string {
	if (!message || message.startsWith('<!doctype html>') || message.startsWith('<html')) return fallback;
	return message;
}
