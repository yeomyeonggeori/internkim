import type { SupabaseClient } from '@supabase/supabase-js';
import type { CalendarEvent } from '../../routes/calendar/embed/calendar-event-persistence';
import type { CalendarParticipant } from '../../routes/calendar/embed/calendar-participants';
import type { Locale } from '../i18n/locale.svelte';
import { approvedLeaveCalendarEvents } from './supabase-calendar-leave';
import { eventReminderLeadOf } from './event-reminder-lead';

export type CalendarMember = { id: string; name: string | null; email: string | null };

type EventRow = {
	id: string;
	title: string;
	note: string | null;
	location: { name?: string } | null;
	starts_at: string;
	ends_at: string;
	is_whole_day: boolean;
	notify_minutes_before: number | null;
	updated_at: string;
	task_participant: { member_id: string }[];
};

const eventSelection =
	'id, title, note, location, starts_at, ends_at, is_whole_day, notify_minutes_before, updated_at, task_participant (member_id)';

export async function companyCalendarEntries(
	caller: SupabaseClient,
	from: Date,
	to: Date,
	timeZone: string,
	locale: Locale = 'ko'
): Promise<CalendarEvent[]> {
	const members = await calendarMembers(caller);
	const [events, leave] = await Promise.all([
		scheduledEvents(caller, from, to, members, timeZone),
		approvedLeaveCalendarEvents(caller, from, to, members, timeZone, locale)
	]);
	return [...events, ...leave].sort((left, right) => left.startISO.localeCompare(right.startISO));
}

export async function calendarMembers(caller: SupabaseClient): Promise<Map<string, CalendarMember>> {
	const members = await caller
		.from('member')
		.select('id, name, email')
		.neq('status', 'withdrawn')
		.returns<CalendarMember[]>();
	if (members.error) throw new Error(members.error.message);
	return new Map(members.data.map((member) => [member.id, member]));
}

export async function calendarEventByID(
	caller: SupabaseClient,
	eventID: string,
	members: Map<string, CalendarMember>,
	timeZone: string
): Promise<CalendarEvent> {
	const event = await caller.from('task').select(eventSelection).eq('id', eventID).single<EventRow>();
	if (event.error) throw new Error(event.error.message);
	return scheduledEvent(event.data, members, timeZone);
}

async function scheduledEvents(
	caller: SupabaseClient,
	from: Date,
	to: Date,
	members: Map<string, CalendarMember>,
	timeZone: string
): Promise<CalendarEvent[]> {
	const events = await caller
		.from('task')
		.select(eventSelection)
		.eq('is_event', true)
		.neq('status', 'rejected')
		.lt('starts_at', to.toISOString())
		.gte('ends_at', from.toISOString())
		.order('starts_at')
		.returns<EventRow[]>();
	if (events.error) throw new Error(events.error.message);
	return events.data.map((event) => scheduledEvent(event, members, timeZone));
}

function scheduledEvent(
	event: EventRow,
	members: Map<string, CalendarMember>,
	timeZone: string
): CalendarEvent {
	return {
		id: event.id,
		uid: event.id,
		title: event.title,
		description: event.note ?? '',
		location: event.location?.name ?? '',
		startISO: event.starts_at,
		endISO: event.ends_at,
		timeZone,
		isAllDay: event.is_whole_day,
		color: '',
		participants: event.task_participant.map((participant) => memberParticipant(participant.member_id, members)),
		createdByEmail: '',
		createdByName: '',
		reminderMinutesBefore: eventReminderLeadOf(event.notify_minutes_before),
		updatedAt: event.updated_at
	};
}

function memberParticipant(
	memberID: string,
	members: Map<string, CalendarMember>
): CalendarParticipant {
	const member = members.get(memberID);
	const email = member?.email ?? '';
	return { personID: memberID, name: member?.name || email.split('@')[0], email: email || undefined };
}
