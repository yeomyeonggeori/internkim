import type { CalendarParticipant, CalendarParticipantInput } from './calendar-participants';
import {
	deleteSupabaseCalendarEvent,
	saveSupabaseCalendarEvent,
	supabaseCalendarEvents
} from '$lib/calendar/supabase-calendar';

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
	reminderMinutesBefore?: number | null;
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

export const calendarEventVersionConflictErrorCode = 'calendar_event_version_conflict';

export type CalendarEventPayload = {
	eventID: string;
	expectedUpdatedAt?: string;
	mutationClientID?: string;
	mutationSequence?: number;
	title: string;
	description: string;
	location: string;
	startsAt: string;
	endsAt: string;
	isAllDay: boolean;
	color: string;
	reminderMinutesBefore: number | null;
	participants: CalendarParticipantInput[];
};

export async function fetchCalendarEvents(startDate: Date, endDate: Date, timeZone?: Promise<string>): Promise<CalendarEvent[]> {
	return supabaseCalendarEvents(startDate, endDate, timeZone);
}

export async function writeCalendarEvent(
	isNewEvent: boolean,
	payload: CalendarEventPayload
): Promise<CalendarEvent> {
	return saveSupabaseCalendarEvent(payload, isNewEvent ? null : payload.eventID);
}

export async function deletePersistedCalendarEvent(
	eventID: string,
	expectedUpdatedAt?: string
): Promise<void> {
	return deleteSupabaseCalendarEvent(eventID, expectedUpdatedAt);
}
