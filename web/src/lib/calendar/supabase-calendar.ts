import { supabase } from '$lib/supabase';
import { sizeOfHours, sizeOfWholeDays } from '$lib/flow/task-sizes';
import type { CalendarEvent, CalendarEventPayload } from '../../routes/calendar/embed/calendar-event-persistence';
import type { CalendarParticipant } from '../../routes/calendar/embed/calendar-participants';
import { approvedLeaveCalendarEvents } from './supabase-calendar-leave';

type MemberRow = { id: string; name: string | null; email: string | null };
type EventRow = {
	id: string;
	title: string;
	note: string | null;
	location: { name?: string } | null;
	starts_at: string;
	ends_at: string;
	is_whole_day: boolean;
	updated_at: string;
	task_participant: { member_id: string }[];
};

export async function supabaseCalendarEvents(startDate: Date, endDate: Date): Promise<CalendarEvent[]> {
	const members = await membersByID();
	const timeZone = await companyTimeZone();
	const [events, leave] = await Promise.all([
		taskCalendarEvents(startDate, endDate, members, timeZone),
		approvedLeaveCalendarEvents(startDate, endDate, members, timeZone)
	]);
	return [...events, ...leave].sort((left, right) => left.startISO.localeCompare(right.startISO));
}

async function taskCalendarEvents(
	startDate: Date,
	endDate: Date,
	members: Map<string, MemberRow>,
	timeZone: string
): Promise<CalendarEvent[]> {
	const events = await supabase()
		.from('task')
		.select('id, title, note, location, starts_at, ends_at, is_whole_day, updated_at, task_participant (member_id)')
		.eq('is_event', true)
		.lt('starts_at', endDate.toISOString())
		.gte('ends_at', startDate.toISOString())
		.order('starts_at')
		.returns<EventRow[]>();
	if (events.error) throw new Error(events.error.message);
	return events.data.map((event) => eventOf(event, members, timeZone));
}

export async function saveSupabaseCalendarEvent(payload: CalendarEventPayload): Promise<CalendarEvent> {
	const client = supabase();
	const fields = {
		title: payload.title,
		note: payload.description || null,
		location: payload.location ? { name: payload.location } : null,
		starts_at: payload.startISO,
		ends_at: payload.endISO,
		is_event: true,
		is_whole_day: payload.isAllDay,
		size: sizeOfEvent(payload.startISO, payload.endISO, payload.isAllDay)
	};

	const saved = payload.eventID
		? await client.from('task').update(fields).eq('id', payload.eventID).select('id').single<{ id: string }>()
		: await client
				.from('task')
				.insert({ ...fields, company_id: await companyIDOfMe() })
				.select('id')
				.single<{ id: string }>();
	if (saved.error) throw new Error(saved.error.message);

	await replaceParticipants(saved.data.id, payload.participants.map((participant) => participant.personID));
	return readEvent(saved.data.id);
}

export async function deleteSupabaseCalendarEvent(eventID: string): Promise<void> {
	const { error } = await supabase().from('task').delete().eq('id', eventID);
	if (error) throw new Error(error.message);
}

export async function supabaseCalendarParticipants(): Promise<CalendarParticipant[]> {
	const members = await membersByID();
	return [...members.values()].map((member) => ({
		personID: member.id,
		name: member.name || (member.email ?? '').split('@')[0],
		email: member.email ?? undefined
	}));
}

async function readEvent(eventID: string): Promise<CalendarEvent> {
	const event = await supabase()
		.from('task')
		.select('id, title, note, location, starts_at, ends_at, is_whole_day, updated_at, task_participant (member_id)')
		.eq('id', eventID)
		.single<EventRow>();
	if (event.error) throw new Error(event.error.message);
	return eventOf(event.data, await membersByID(), await companyTimeZone());
}

function eventOf(event: EventRow, members: Map<string, MemberRow>, timeZone: string): CalendarEvent {
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
		participants: event.task_participant.map((participant) => participantOf(participant.member_id, members)),
		createdByEmail: '',
		createdByName: '',
		updatedAt: event.updated_at
	};
}

function participantOf(memberID: string, members: Map<string, MemberRow>): CalendarParticipant {
	const member = members.get(memberID);
	const email = member?.email ?? '';
	return { personID: memberID, name: member?.name || email.split('@')[0], email: email || undefined };
}

async function replaceParticipants(taskID: string, participantIDs: string[]): Promise<void> {
	const client = supabase();
	const removed = await client.from('task_participant').delete().eq('task_id', taskID);
	if (removed.error) throw new Error(removed.error.message);
	if (participantIDs.length === 0) return;
	const added = await client
		.from('task_participant')
		.insert(participantIDs.map((memberID) => ({ task_id: taskID, member_id: memberID })));
	if (added.error) throw new Error(added.error.message);
}

function sizeOfEvent(startISO: string, endISO: string, isAllDay: boolean): string {
	const hours = (new Date(endISO).getTime() - new Date(startISO).getTime()) / 3600000;
	if (!isAllDay) return sizeOfHours(hours);
	return sizeOfWholeDays(Math.max(1, Math.round(hours / 24)));
}

async function membersByID(): Promise<Map<string, MemberRow>> {
	const members = await supabase()
		.from('member')
		.select('id, name, email')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);
	return new Map(members.data.map((member) => [member.id, member]));
}

async function companyTimeZone(): Promise<string> {
	const company = await supabase().from('company').select('timezone').limit(1).single<{ timezone: string }>();
	if (company.error) throw new Error(company.error.message);
	return company.data.timezone;
}

async function companyIDOfMe(): Promise<string> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id;
	if (!accountID) throw new Error('sign in first');
	const member = await client
		.from('member')
		.select('company_id')
		.eq('user_id', accountID)
		.single<{ company_id: string }>();
	if (member.error) throw new Error(member.error.message);
	return member.data.company_id;
}
