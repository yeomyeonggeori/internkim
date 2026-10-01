import type { SupabaseClient } from '@supabase/supabase-js';
import { dayOffColor } from '$lib/calendar/day-off-color';
import { eventReminderLeadOf } from '$lib/calendar/event-reminder-lead';
import { localizedLeaveUnitName } from '$lib/i18n/leave-type-name';
import { personName } from '$lib/person-name';
import type { Locale } from '$lib/i18n/locale';
import type { RecordPerson } from './people';

export type CompanyCalendarSource = 'event' | 'leave';

export type CompanyCalendarParticipant = {
	personID: string;
	name: string;
	email?: string;
};

export type CompanyCalendarEntry = {
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
	reminderMinutesBefore: number | null;
	participants: CompanyCalendarParticipant[];
	createdByEmail: string;
	createdByName: string;
	updatedAt: string;
	readOnly: boolean;
	source: CompanyCalendarSource;
};

export type CompanyCalendarMember = {
	id: string;
	name: string | null;
	email: string | null;
};

export type CompanyCalendarReader = {
	caller: SupabaseClient;
	members: Map<string, CompanyCalendarMember>;
	timeZone: string;
	locale: Locale;
};

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

export type ApprovedLeaveRow = {
	id: string;
	member_id: string;
	kind: string;
	days: number;
	status: 'approved';
	starts_at: string;
	ends_at: string;
};

const eventSelection =
	'id, title, note, location, starts_at, ends_at, is_whole_day, notify_minutes_before, updated_at, task_participant (member_id)';

const leaveSelection = 'id, member_id, kind, days, status, starts_at, ends_at';

const wholeDayLeaveThreshold = 0.5;

export async function companyCalendarEntries(
	reader: CompanyCalendarReader,
	from: Date,
	to: Date
): Promise<CompanyCalendarEntry[]> {
	const [events, leave] = await Promise.all([
		scheduledEvents(reader, from, to),
		approvedLeave(reader, from, to)
	]);
	return [...events, ...leave].sort((left, right) => left.startISO.localeCompare(right.startISO));
}

export async function calendarMembers(
	caller: SupabaseClient
): Promise<Map<string, CompanyCalendarMember>> {
	const members = await caller
		.from('member')
		.select('id, name, email')
		.neq('status', 'withdrawn')
		.returns<CompanyCalendarMember[]>();
	if (members.error) throw new Error(members.error.message);
	return new Map(members.data.map((member) => [member.id, member]));
}

export function calendarMembersOfPeople(
	people: RecordPerson[]
): Map<string, CompanyCalendarMember> {
	return new Map(
		people.map((person) => [person.personID, { id: person.personID, name: person.name, email: person.email }])
	);
}

async function scheduledEvents(
	reader: CompanyCalendarReader,
	from: Date,
	to: Date
): Promise<CompanyCalendarEntry[]> {
	const events = await reader.caller
		.from('task')
		.select(eventSelection)
		.eq('is_event', true)
		.neq('status', 'rejected')
		.lt('starts_at', to.toISOString())
		.gte('ends_at', from.toISOString())
		.order('starts_at')
		.returns<EventRow[]>();
	if (events.error) throw new Error(`Failed to load company calendar events: ${events.error.message}`);
	return events.data.map((event) => calendarEntryOfEvent(event, reader));
}

async function approvedLeave(
	reader: CompanyCalendarReader,
	from: Date,
	to: Date
): Promise<CompanyCalendarEntry[]> {
	const leave = await reader.caller
		.from('leave')
		.select(leaveSelection)
		.eq('status', 'approved')
		.lt('days', 0)
		.lt('starts_at', to.toISOString())
		.gte('ends_at', from.toISOString())
		.order('starts_at')
		.returns<ApprovedLeaveRow[]>();
	if (leave.error) throw new Error(`Failed to load approved leave calendar events: ${leave.error.message}`);
	return leave.data.map((row) =>
		calendarEntryOfApprovedLeave({ ...row, days: -row.days }, reader.members, reader.timeZone, reader.locale)
	);
}

function calendarEntryOfEvent(event: EventRow, reader: CompanyCalendarReader): CompanyCalendarEntry {
	return {
		id: event.id,
		uid: event.id,
		title: event.title,
		description: event.note ?? '',
		location: event.location?.name ?? '',
		startISO: event.starts_at,
		endISO: event.ends_at,
		timeZone: reader.timeZone,
		isAllDay: event.is_whole_day,
		color: '',
		reminderMinutesBefore: eventReminderLeadOf(event.notify_minutes_before),
		participants: event.task_participant.map((participant) =>
			calendarParticipant(participant.member_id, reader.members, reader.locale)
		),
		createdByEmail: '',
		createdByName: '',
		updatedAt: event.updated_at,
		readOnly: false,
		source: 'event'
	};
}

export function calendarEntryOfApprovedLeave(
	leave: ApprovedLeaveRow,
	members: Map<string, CompanyCalendarMember>,
	timeZone: string,
	locale: Locale = 'ko'
): CompanyCalendarEntry {
	const member = members.get(leave.member_id);
	const email = member?.email ?? '';
	const name = personName(member?.name || email.split('@')[0] || anonymousMemberName(locale), locale);
	const id = `leave:${leave.id}`;
	const isAllDay = leave.days > wholeDayLeaveThreshold;
	return {
		id,
		uid: id,
		title: `${name} · ${leaveKindLabel(leave.kind, leave.days, locale)}`,
		description: '',
		location: '',
		startISO: leave.starts_at,
		endISO: leave.ends_at,
		timeZone,
		isAllDay,
		color: dayOffColor,
		reminderMinutesBefore: null,
		participants: [{ personID: leave.member_id, name, ...(email ? { email } : {}) }],
		createdByEmail: email,
		createdByName: name,
		updatedAt: leave.starts_at,
		readOnly: true,
		source: 'leave'
	};
}

function anonymousMemberName(locale: Locale): string {
	return locale === 'ko' ? '구성원' : 'Member';
}

function calendarParticipant(
	memberID: string,
	members: Map<string, CompanyCalendarMember>,
	locale: Locale
): CompanyCalendarParticipant {
	const member = members.get(memberID);
	const email = member?.email ?? '';
	const name = personName(member?.name || email.split('@')[0], locale);
	return { personID: memberID, name, ...(email ? { email } : {}) };
}

const kindsNamedByTheirLength = new Set(['annual', 'leave', '연차', '반차']);

function leaveKindLabel(kind: string, days: number, locale: Locale): string {
	if (!kindsNamedByTheirLength.has(kind)) return kind;
	return localizedLeaveUnitName(days, locale);
}
