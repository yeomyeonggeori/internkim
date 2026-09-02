import { supabase } from '$lib/supabase';
import { sizeOfHours, sizeOfWholeDays } from '$lib/task/task-sizes';
import type { CalendarEvent, CalendarEventPayload } from '../../routes/calendar/embed/calendar-event-persistence';
import type { CalendarParticipant } from '../../routes/calendar/embed/calendar-participants';
import type { Locale } from '../i18n/locale.svelte';
import { calendarEventByID, calendarMembers, companyCalendarEntries } from './company-calendar';

export async function supabaseCalendarEvents(
	startDate: Date,
	endDate: Date,
	locale: Locale = 'ko'
): Promise<CalendarEvent[]> {
	return companyCalendarEntries(supabase(), startDate, endDate, await companyTimeZone(), locale);
}

export async function saveSupabaseCalendarEvent(
	payload: CalendarEventPayload,
	targetEventID: string | null
): Promise<CalendarEvent> {
	const saved = await supabase().rpc('task_save', supabaseCalendarEventRPCArguments(payload, targetEventID));
	if (saved.error) throw new Error(saved.error.message);
	if (typeof saved.data !== 'string') throw new Error('calendar event save returned no event ID');
	return readEvent(saved.data);
}

export type SupabaseCalendarEventRPCArguments = {
	target_task_id: string | null;
	target_title: string;
	target_note: string | null;
	target_location: { name: string } | null;
	target_starts_at: string;
	target_ends_at: string;
	target_is_whole_day: boolean;
	target_is_event: true;
	target_size: string;
	target_participant_ids: string[];
};

export function supabaseCalendarEventRPCArguments(
	payload: CalendarEventPayload,
	targetEventID: string | null
): SupabaseCalendarEventRPCArguments {
	return {
		target_task_id: targetEventID,
		target_title: payload.title,
		target_note: payload.description || null,
		target_location: payload.location ? { name: payload.location } : null,
		target_starts_at: payload.startISO,
		target_ends_at: payload.endISO,
		target_is_whole_day: payload.isAllDay,
		target_is_event: true,
		target_size: sizeOfEvent(payload.startISO, payload.endISO, payload.isAllDay),
		target_participant_ids: payload.participants.map((participant) => participant.personID)
	};
}

export async function deleteSupabaseCalendarEvent(eventID: string): Promise<void> {
	const { error } = await supabase()
		.from('task')
		.delete()
		.eq('id', eventID)
		.select('id')
		.single<{ id: string }>();
	if (error) throw new Error(error.message);
}

export async function supabaseCalendarParticipants(): Promise<CalendarParticipant[]> {
	const members = await calendarMembers(supabase());
	return [...members.values()].map((member) => ({
		personID: member.id,
		name: member.name || (member.email ?? '').split('@')[0],
		email: member.email ?? undefined
	}));
}

async function readEvent(eventID: string): Promise<CalendarEvent> {
	const client = supabase();
	return calendarEventByID(client, eventID, await calendarMembers(client), await companyTimeZone());
}

function sizeOfEvent(startISO: string, endISO: string, isAllDay: boolean): string {
	const hours = (new Date(endISO).getTime() - new Date(startISO).getTime()) / 3600000;
	if (!isAllDay) return sizeOfHours(hours);
	return sizeOfWholeDays(Math.max(1, Math.round(hours / 24)));
}

async function companyTimeZone(): Promise<string> {
	const company = await supabase().from('company').select('timezone').limit(1).single<{ timezone: string }>();
	if (company.error) throw new Error(company.error.message);
	return company.data.timezone;
}
