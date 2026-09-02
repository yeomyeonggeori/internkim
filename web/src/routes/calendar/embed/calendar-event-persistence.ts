import type { CalendarParticipant, CalendarParticipantInput } from './calendar-participants';
import {
	deleteSupabaseCalendarEvent,
	saveSupabaseCalendarEvent,
	supabaseCalendarEvents
} from '$lib/calendar/supabase-calendar';
import { isSupabaseConfigured } from '$lib/supabase';
import { calendarDeleteUndoTimeoutMs } from './calendar-delete-window';
import type { Locale } from '$lib/i18n/locale.svelte';

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
	readOnly?: boolean;
	source?: string;
};

export type CalendarEventPayload = {
	eventID: string;
	expectedUpdatedAt?: string;
	mutationClientID?: string;
	mutationSequence?: number;
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

export type CalendarDeleteIntent = {
	operationID: string;
	executeAt: string;
};

export const calendarEventVersionConflictErrorCode = 'calendar_event_version_conflict';
export const calendarDeleteIntentConflictErrorCode = 'calendar_delete_intent_conflict';

export type CalendarPersistenceErrorCode =
	| typeof calendarEventVersionConflictErrorCode
	| typeof calendarDeleteIntentConflictErrorCode
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

export function isCalendarPersistenceErrorCode(
	error: unknown,
	code: CalendarPersistenceErrorCode
): error is CalendarPersistenceError {
	return error instanceof CalendarPersistenceError && error.code === code;
}

export async function fetchCalendarEvents(
	startDate: Date,
	endDate: Date,
	errorFallback: string,
	locale: Locale = 'ko'
): Promise<CalendarEvent[]> {
	if (isSupabaseConfigured()) return supabaseCalendarEvents(startDate, endDate, locale);
	const query = new URLSearchParams({
		startISO: startDate.toISOString(),
		endISO: endDate.toISOString(),
		locale
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
	if (isSupabaseConfigured()) return saveSupabaseCalendarEvent(payload, method === 'POST' ? null : payload.eventID);
	const response = await fetch(path, {
		method,
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(payload)
	});
	if (!response.ok) throw await calendarPersistenceErrorFromResponse(response, errorFallback);
	return (await response.json()) as CalendarEvent;
}

export function calendarEventPath(eventID: string): string {
	return `/calendar/api/events/${encodeURIComponent(eventID)}`;
}

export async function deletePersistedCalendarEvent(
	eventID: string,
	expectedUpdatedAt: string | undefined,
	errorFallback: string
): Promise<void> {
	if (isSupabaseConfigured()) return deleteSupabaseCalendarEvent(eventID);
	const response = await fetch(calendarEventPath(eventID), {
		method: 'DELETE',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ expectedUpdatedAt })
	});
	if (!response.ok) throw await calendarPersistenceErrorFromResponse(response, errorFallback);
}

export async function createCalendarEventDeleteIntent(
	eventID: string,
	operationID: string,
	clientID: string,
	sequence: number,
	expectedUpdatedAt: string,
	errorFallback: string
): Promise<CalendarDeleteIntent> {
	if (isSupabaseConfigured()) return browserHeldDeleteIntent(operationID);
	const response = await fetch(calendarDeleteIntentPath(eventID, operationID), {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ clientID, sequence, expectedUpdatedAt }),
		keepalive: true
	});
	if (!response.ok) throw await calendarPersistenceErrorFromResponse(response, errorFallback);
	return parseCalendarDeleteIntent(await response.json(), operationID, errorFallback);
}

export async function cancelCalendarEventDeleteIntent(
	eventID: string,
	operationID: string,
	clientID: string,
	sequence: number,
	errorFallback: string
): Promise<void> {
	if (isSupabaseConfigured()) return;
	const response = await fetch(calendarDeleteIntentPath(eventID, operationID), {
		method: 'DELETE',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ clientID, sequence }),
		keepalive: true
	});
	if (!response.ok) throw await calendarPersistenceErrorFromResponse(response, errorFallback);
}

export function browserHoldsDeleteIntent(): boolean {
	return isSupabaseConfigured();
}

// A company is reached from browsers this device cannot see, so the wait before
// a delete becomes real is held here rather than queued on the device.
function browserHeldDeleteIntent(operationID: string): CalendarDeleteIntent {
	return {
		operationID,
		executeAt: new Date(Date.now() + calendarDeleteUndoTimeoutMs).toISOString()
	};
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
	if (document.code === calendarEventVersionConflictErrorCode) return calendarEventVersionConflictErrorCode;
	if (document.code === calendarDeleteIntentConflictErrorCode) return calendarDeleteIntentConflictErrorCode;
	return 'unknown';
}

function parseCalendarDeleteIntent(
	document: unknown,
	expectedOperationID: string,
	fallback: string
): CalendarDeleteIntent {
	if (!isCalendarDeleteIntent(document, expectedOperationID)) {
		throw new CalendarPersistenceError('unknown', fallback);
	}
	return document;
}

function isCalendarDeleteIntent(
	document: unknown,
	expectedOperationID: string
): document is CalendarDeleteIntent {
	if (!document || typeof document !== 'object') return false;
	if (!('operationID' in document) || !('executeAt' in document)) return false;
	if (document.operationID !== expectedOperationID || typeof document.executeAt !== 'string') return false;
	return document.executeAt.trim() !== '' && !Number.isNaN(Date.parse(document.executeAt));
}

function responseBodyErrorMessage(message: string, fallback: string): string {
	if (!message || message.startsWith('<!doctype html>') || message.startsWith('<html')) return fallback;
	return message;
}

function calendarDeleteIntentPath(eventID: string, operationID: string): string {
	return `${calendarEventPath(eventID)}/delete-intents/${encodeURIComponent(operationID)}`;
}
