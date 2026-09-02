import { companySettings } from '$lib/company/company-settings';
import { invokeTool } from '$lib/public-api-call';
import { supabase } from '$lib/supabase';
import type { CalendarEvent, CalendarEventPayload } from '../../routes/calendar/embed/calendar-event-persistence';
import type { CalendarParticipant } from '../../routes/calendar/embed/calendar-participants';
import type { Locale } from '../i18n/locale.svelte';
import { companyCalendarEntries } from './company-calendar';

type AnsweredAttendee = { personID?: string; name: string; email?: string };

type AnsweredEvent = {
	eventID: string;
	title: string;
	note: string;
	location: string;
	startsAt: string;
	endsAt: string;
	isWholeDay: boolean;
	participants: AnsweredAttendee[];
	updatedAt: string;
};

type AnsweredPeople = { people: { personID: string; name: string; email: string }[] };

export async function supabaseCalendarEvents(
	startDate: Date,
	endDate: Date,
	locale: Locale = 'ko'
): Promise<CalendarEvent[]> {
	return companyCalendarEntries(supabase(), startDate, endDate, await companyTimeZone(), locale);
}

export function calendarEventWritten(payload: CalendarEventPayload): Record<string, unknown> {
	return {
		title: payload.title,
		note: payload.description,
		location: payload.location,
		startsAt: payload.startISO,
		endsAt: payload.endISO,
		isWholeDay: payload.isAllDay,
		participantPersonHints: payload.participants.map((participant) => participant.personID)
	};
}

export async function saveSupabaseCalendarEvent(
	payload: CalendarEventPayload,
	targetEventID: string | null
): Promise<CalendarEvent> {
	const written = calendarEventWritten(payload);
	const saved = targetEventID
		? await invokeTool<AnsweredEvent>('event_update', { eventHint: targetEventID, ...written })
		: await invokeTool<AnsweredEvent>('event_add', written);
	return calendarEventFromAnswer(saved, await companyTimeZone());
}

export async function deleteSupabaseCalendarEvent(eventID: string): Promise<void> {
	await invokeTool('event_delete', { eventHint: eventID });
}

export async function supabaseCalendarParticipants(): Promise<CalendarParticipant[]> {
	const answered = await invokeTool<AnsweredPeople>('person_list', {});
	return answered.people.map((person) => ({
		personID: person.personID,
		name: person.name || person.email.split('@')[0],
		email: person.email || undefined
	}));
}

export function calendarEventFromAnswer(answered: AnsweredEvent, timeZone: string): CalendarEvent {
	return {
		id: answered.eventID,
		uid: answered.eventID,
		title: answered.title,
		description: answered.note,
		location: answered.location,
		startISO: answered.startsAt,
		endISO: answered.endsAt,
		timeZone,
		isAllDay: answered.isWholeDay,
		color: '',
		participants: answered.participants.map((attendee) => ({
			personID: attendee.personID ?? '',
			name: attendee.name,
			email: attendee.email
		})),
		createdByEmail: '',
		createdByName: '',
		updatedAt: answered.updatedAt
	};
}

async function companyTimeZone(): Promise<string> {
	return (await companySettings()).timeZone;
}
