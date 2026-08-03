// Imports device work items and calendar events into task. Reads credentials from .env.
//   bun run web/scripts/import-tasks.ts --sqlite <copy> --company <uuid> [--apply]
//
// Work items and events are one table here, told apart by is_event. Owners become
// participants, since a task has no owner. Attendees who are not members cannot be
// represented and are named in the note instead of being silently dropped.

import { Database } from 'bun:sqlite';
import { controlPlane } from '../src/lib/server/control-plane';

type DeviceFlowTask = {
	id: string;
	owner_name: string | null;
	participant_names: string | null;
	content: string | null;
	goal: string | null;
	status: string | null;
	start_date: string | null;
	end_date: string | null;
	request_reason: string | null;
	created_at: string | null;
};

type DeviceCalendarEvent = {
	id: string;
	title: string | null;
	description: string | null;
	location: string | null;
	start_at: string;
	end_at: string;
	is_all_day: number;
	reminder_lead_hours: number | null;
	created_by_email: string | null;
	deleted_at: string | null;
};

type DeviceParticipant = { event_id: string; email: string | null; name: string | null };

const STATUS_OF_DEVICE: Record<string, string> = {
	예정: 'todo',
	진행: 'in_progress',
	완료: 'done',
	중단: 'cancelled',
	기각: 'cancelled',
	일시정지: 'paused',
};

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const sqlitePath = argument('sqlite');
const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!sqlitePath || !companyID) throw new Error('pass --sqlite <path> --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data: members, error: memberError } = await client
	.from('member')
	.select('id, email, user_id')
	.eq('company_id', companyID);
if (memberError) throw new Error(memberError.message);
const memberByEmail = new Map((members ?? []).map((member) => [member.email ?? '', member.id]));

// Device work items carry a display name, not an address. Names are matched exactly
// and only when a single member answers to one; anything else is left unassigned.
const { data: accounts } = await client.auth.admin.listUsers();
const memberByName = new Map<string, string>();
const ambiguousNames = new Set<string>();
for (const account of accounts.users) {
	const name = (account.user_metadata?.full_name ?? '').trim();
	const memberID = memberByEmail.get(account.email ?? '');
	if (!name || !memberID) continue;
	if (memberByName.has(name)) ambiguousNames.add(name);
	memberByName.set(name, memberID);
}
for (const name of ambiguousNames) memberByName.delete(name);

const device = new Database(sqlitePath);
const flowTasks = device
	.query(
		'select id, owner_name, participant_names, content, goal, status, start_date, end_date, request_reason, created_at from flow_tasks',
	)
	.all() as DeviceFlowTask[];
const events = device
	.query(
		'select id, title, description, location, start_at, end_at, is_all_day, reminder_lead_hours, created_by_email, deleted_at from calendar_events',
	)
	.all() as DeviceCalendarEvent[];
const participants = device
	.query('select event_id, email, name from calendar_event_participants')
	.all() as DeviceParticipant[];

const liveEvents = events.filter((event) => !(event.deleted_at ?? '').trim());
const participantsByEvent = new Map<string, DeviceParticipant[]>();
for (const participant of participants) {
	participantsByEvent.set(participant.event_id, [
		...(participantsByEvent.get(participant.event_id) ?? []),
		participant,
	]);
}

const unknownStatuses = new Set(
	flowTasks.map((task) => (task.status ?? '').trim()).filter((status) => status && !STATUS_OF_DEVICE[status]),
);
const unresolvedOwners = new Set(
	flowTasks.map((task) => (task.owner_name ?? '').trim()).filter((name) => name && !memberByName.has(name)),
);
const outsiders = participants.filter((participant) => !memberByEmail.has(participant.email ?? ''));

console.log(`work items ${flowTasks.length}`);
console.log(`events ${events.length}, live ${liveEvents.length}, with a location ${liveEvents.filter((event) => (event.location ?? '').trim()).length}`);
console.log(`attendees ${participants.length}, of whom members ${participants.length - outsiders.length}, named in notes ${outsiders.length}`);
if (unknownStatuses.size) console.log(`statuses with no mapping: ${[...unknownStatuses].join(', ')}`);
if (unresolvedOwners.size) console.log(`names that match no single member: ${[...unresolvedOwners].join(', ')}`);

if (!shouldApply) {
	console.log('\ndry run — pass --apply to write');
	process.exit(0);
}

// A few device rows hold a full timestamp where a date belongs, so take the day
// off the front and ignore anything that is not one.
function dayOf(value: string | null): string {
	const day = (value ?? '').trim().slice(0, 10);
	return /^\d{4}-\d{2}-\d{2}$/.test(day) ? day : '';
}

function dayRangeOf(startDate: string | null, endDate: string | null): { startsAt: string; endsAt: string } | null {
	const start = dayOf(startDate);
	if (!start) return null;
	// The record puts a reversed range in order, so both days go across as given.
	const end = dayOf(endDate) || start;
	return { startsAt: `${start}T00:00:00+09:00`, endsAt: `${end}T23:59:00+09:00` };
}

function participantNamesOf(raw: string | null): string[] {
	try {
		const parsed: unknown = JSON.parse(raw ?? '[]');
		return Array.isArray(parsed) ? parsed.filter((name): name is string => typeof name === 'string') : [];
	} catch {
		return [];
	}
}

let writtenTasks = 0;
for (const task of flowTasks) {
	const title = (task.content ?? '').trim() || '(제목 없음)';
	const range = dayRangeOf(task.start_date, task.end_date);
	const noteParts = [task.goal?.trim() && `목표: ${task.goal.trim()}`, task.request_reason?.trim()].filter(Boolean);
	const { data, error } = await client
		.from('task')
		.insert({
			company_id: companyID,
			title,
			status: STATUS_OF_DEVICE[(task.status ?? '').trim()] ?? 'todo',
			note: noteParts.join('\n') || null,
			// A range needs both ends, and the device has rows with only one of them.
			starts_at: range?.startsAt ?? null,
			ends_at: range?.endsAt ?? null,
			is_whole_day: Boolean(range),
		})
		.select('id')
		.single();
	if (error) throw new Error(`task ${task.id}: ${error.message}`);
	writtenTasks += 1;

	const names = new Set([...(task.owner_name ? [task.owner_name.trim()] : []), ...participantNamesOf(task.participant_names)]);
	const memberIDs = [...names].map((name) => memberByName.get(name)).filter((id): id is string => Boolean(id));
	if (memberIDs.length) {
		const { error: participantError } = await client
			.from('task_participant')
			.upsert(memberIDs.map((memberID) => ({ task_id: data.id, member_id: memberID })));
		if (participantError) throw new Error(`task participants ${task.id}: ${participantError.message}`);
	}
}
console.log(`wrote ${writtenTasks} work items`);

let writtenEvents = 0;
for (const event of liveEvents) {
	const attendees = participantsByEvent.get(event.id) ?? [];
	const guests = attendees.filter((attendee) => !memberByEmail.has(attendee.email ?? ''));
	const noteParts = [
		event.description?.trim(),
		guests.length ? `외부 참석자: ${guests.map((guest) => guest.name || guest.email).join(', ')}` : '',
	].filter(Boolean);
	const location = (event.location ?? '').trim();

	const { data, error } = await client
		.from('task')
		.insert({
			company_id: companyID,
			title: (event.title ?? '').trim() || '(제목 없음)',
			is_event: true,
			is_whole_day: Boolean(event.is_all_day),
			starts_at: event.start_at,
			ends_at: event.end_at,
			location: location ? { name: location } : null,
			notify_minutes_before: event.reminder_lead_hours ? event.reminder_lead_hours * 60 : null,
			note: noteParts.join('\n') || null,
			status: 'done',
		})
		.select('id')
		.single();
	if (error) throw new Error(`event ${event.id}: ${error.message}`);
	writtenEvents += 1;

	const memberIDs = attendees
		.map((attendee) => memberByEmail.get(attendee.email ?? ''))
		.filter((id): id is string => Boolean(id));
	const creatorID = memberByEmail.get(event.created_by_email ?? '');
	const everyone = [...new Set([...memberIDs, ...(creatorID ? [creatorID] : [])])];
	if (everyone.length) {
		const { error: participantError } = await client
			.from('task_participant')
			.upsert(everyone.map((memberID) => ({ task_id: data.id, member_id: memberID })));
		if (participantError) throw new Error(`event participants ${event.id}: ${participantError.message}`);
	}
}
console.log(`wrote ${writtenEvents} events`);
