//   bun run web/scripts/import-calendar-events.ts --file <calendar-events.json> --people <organization-people.json> --company <uuid> [--apply]

import { controlPlane } from '../src/lib/server/control-plane';
import {
	emailByPersonIDOf,
	eventAsTask,
	matchParticipant,
	timeRunsBackwards,
	titleOf,
	type DeviceCalendarEvent,
	type DeviceCalendarParticipant,
	type DevicePerson,
	type EventCalendar,
	type ParticipantMatch
} from '../../host/relay/calendar-event-as-task';

type LiveEvent = { id: string; title: string; starts_at: string; calendar: unknown };

type ResolvedParticipant = { memberID: string; email: string; by: ParticipantMatch['by'] };

type EventFields = { title: string; starts_at: string; calendar: EventCalendar };

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const file = argument('file');
const peopleFile = argument('people');
const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!file || !peopleFile || !companyID) {
	throw new Error('pass --file <calendar-events.json> --people <organization-people.json> --company <uuid>');
}

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? ''
});

const events = (JSON.parse(await Bun.file(file).text()) as { events?: DeviceCalendarEvent[] }).events ?? [];
const devicePeople = (JSON.parse(await Bun.file(peopleFile).text()) as { records?: DevicePerson[] }).records ?? [];

const { data: memberRows, error: memberError } = await client
	.from('member')
	.select('id, email')
	.eq('company_id', companyID);
if (memberError) throw new Error(memberError.message);
const memberByEmail = new Map((memberRows ?? []).map((member) => [(member.email ?? '').toLowerCase(), member.id]));

const emailByPersonID = emailByPersonIDOf(devicePeople);

const { data: existing, error: eventError } = await client
	.from('task')
	.select('id, title, starts_at, calendar')
	.eq('company_id', companyID)
	.eq('is_event', true);
if (eventError) throw new Error(eventError.message);
const liveEvents = (existing ?? []) as LiveEvent[];

const idByExternalID = new Map<string, string>();
for (const task of liveEvents) {
	for (const mirror of (task.calendar as EventCalendar | null)?.mirrors ?? []) {
		idByExternalID.set(mirror.externalID, task.id);
	}
}

const { data: participantRows, error: participantReadError } = await client
	.from('task_participant')
	.select('task_id, member_id')
	.in('task_id', liveEvents.map((live) => live.id));
if (participantReadError) throw new Error(participantReadError.message);
const peopleByTaskID = new Map<string, Set<string>>();
for (const row of participantRows ?? []) {
	const already = peopleByTaskID.get(row.task_id) ?? new Set<string>();
	already.add(row.member_id);
	peopleByTaskID.set(row.task_id, already);
}

const deviceTitleCounts = countTitles(events.map((event) => event.title));
const claimedIDs = new Set<string>();

let adopted = 0;
let updated = 0;
let inserted = 0;
const skippedTitles: string[] = [];
const unresolvedParticipants = new Set<string>();
const matchedByNameOrHandle = new Set<string>();
const matchedByGivenName = new Set<string>();
const creatorsMatchingNobody = new Set<string>();
const refusedByTheRecord: string[] = [];
const heldTwiceByTheDevice: string[] = [];

for (const event of events) {
	const title = titleOf(event);

	if (timeRunsBackwards(event)) {
		refusedByTheRecord.push(`${title}: ends ${event.endISO} before it starts ${event.startISO}`);
		continue;
	}

	const writtenParticipants = event.participants ?? [];
	const attendeeIDs = new Set<string>();
	for (const participant of writtenParticipants) {
		const found = resolveParticipant(participant);
		if (!found) {
			unresolvedParticipants.add(participant.name?.trim() || '(이름 없음)');
			continue;
		}
		attendeeIDs.add(found.memberID);
		if (found.by === 'nameOrHandle') matchedByNameOrHandle.add(`${participant.name?.trim()} → ${found.email}`);
		if (found.by === 'givenName') matchedByGivenName.add(`${participant.name?.trim()} → ${found.email}`);
	}

	const createdByEmail = (event.createdByEmail ?? '').toLowerCase();
	const requesterID = memberByEmail.get(createdByEmail);
	if (createdByEmail && !requesterID) creatorsMatchingNobody.add(createdByEmail);

	if (!requesterID && attendeeIDs.size === 0 && writtenParticipants.length > 0) {
		skippedTitles.push(title);
		continue;
	}

	const asTask = eventAsTask(event);
	const fields = {
		company_id: companyID,
		title: asTask.title,
		is_event: true,
		is_whole_day: asTask.isWholeDay,
		starts_at: asTask.startsAt,
		ends_at: asTask.endsAt,
		note: asTask.note,
		location: asTask.location,
		notify_minutes_before: asTask.notifyMinutesBefore,
		calendar: asTask.calendar
	};

	const mirrored = idByExternalID.get(event.uid) ?? idByExternalID.get(event.id);
	const known = mirrored ?? adoptEventImportedBeforeMirrors(event);
	if (known) claimedIDs.add(known);
	if (known && !mirrored) adopted += 1;

	if (!known && sameEventIsAlreadyHere(fields, attendeeIDs)) {
		heldTwiceByTheDevice.push(`${title} ${event.startISO}`);
		continue;
	}

	if (!shouldApply) {
		known ? (updated += 1) : (inserted += 1);
		continue;
	}

	const written = known
		? await client.from('task').update(fields).eq('id', known).select('id').single()
		: await client.from('task').insert({ ...fields, requester_id: requesterID ?? null }).select('id').single();
	if (written.error) throw new Error(`${title}: ${written.error.message}`);
	known ? (updated += 1) : (inserted += 1);
	rememberWhatIsHere(written.data.id, fields, attendeeIDs);

	if (attendeeIDs.size === 0) continue;
	const { error: participantError } = await client
		.from('task_participant')
		.upsert([...attendeeIDs].map((memberID) => ({ task_id: written.data.id, member_id: memberID })));
	if (participantError) throw new Error(`${title} participants: ${participantError.message}`);
}

console.log(`${shouldApply ? 'wrote' : 'would write'}: ${updated} updated, ${inserted} inserted, ${skippedTitles.length} skipped`);
if (adopted) console.log(`adopted ${adopted} events that an earlier import left without a mirror`);
if (skippedTitles.length) console.log(`skipped, nobody in this company asked for them or is on them: ${skippedTitles.join(', ')}`);
if (matchedByNameOrHandle.size) console.log(`matched by an exact name or handle: ${[...matchedByNameOrHandle].join(', ')}`);
if (matchedByGivenName.size) console.log(`matched by a unique given name: ${[...matchedByGivenName].join(', ')}`);
if (unresolvedParticipants.size) console.log(`written on an event but not a member, left off: ${[...unresolvedParticipants].join(', ')}`);
if (creatorsMatchingNobody.size) console.log(`created by an address no member holds, kept without a requester: ${[...creatorsMatchingNobody].join(', ')}`);
if (heldTwiceByTheDevice.length) console.log(`the device holds these twice, so only the first came across: ${heldTwiceByTheDevice.join(', ')}`);
for (const refusal of refusedByTheRecord) console.log(`refused by the record: ${refusal}`);

// One event per title, time and people is what the record allows, so a second
// copy is refused here rather than left for the database to reject halfway
// through writing it. The device holds a few of these under separate uids.
function sameEventIsAlreadyHere(fields: EventFields, attendeeIDs: Set<string>): boolean {
	return liveEvents.some(
		(live) =>
			live.title.trim() === fields.title &&
			Date.parse(live.starts_at) === Date.parse(fields.starts_at) &&
			samePeople(peopleByTaskID.get(live.id) ?? new Set(), attendeeIDs)
	);
}

function samePeople(here: Set<string>, arriving: Set<string>): boolean {
	return here.size === arriving.size && [...arriving].every((memberID) => here.has(memberID));
}

function rememberWhatIsHere(taskID: string, fields: EventFields, attendeeIDs: Set<string>): void {
	if (!liveEvents.some((live) => live.id === taskID)) {
		liveEvents.push({ id: taskID, title: fields.title, starts_at: fields.starts_at, calendar: fields.calendar });
	}
	peopleByTaskID.set(taskID, new Set(attendeeIDs));
}

// This calendar was imported once before mirrors existed, so those rows carry no
// external id to match on. They are adopted by what the two copies do share, and
// stamped with a mirror as they are updated, which is why this only ever runs on
// the first pass. Each row is claimed once so two events cannot adopt the same one.
function adoptEventImportedBeforeMirrors(event: DeviceCalendarEvent): string | undefined {
	const unclaimed = liveEvents.filter((live) => !claimedIDs.has(live.id) && !hasMirror(live));
	const title = event.title?.trim() || '(제목 없음)';

	const sameMoment = unclaimed.filter(
		(live) => live.title.trim() === title && Date.parse(live.starts_at) === Date.parse(event.startISO)
	);
	if (sameMoment.length === 1) return sameMoment[0].id;

	if (deviceTitleCounts.get(title) !== 1) return undefined;
	const sameTitle = unclaimed.filter((live) => live.title.trim() === title);
	return sameTitle.length === 1 ? sameTitle[0].id : undefined;
}

function hasMirror(live: LiveEvent): boolean {
	return ((live.calendar as EventCalendar | null)?.mirrors ?? []).length > 0;
}

function countTitles(titles: string[]): Map<string, number> {
	const counts = new Map<string, number>();
	for (const title of titles) {
		const trimmed = title?.trim() || '(제목 없음)';
		counts.set(trimmed, (counts.get(trimmed) ?? 0) + 1);
	}
	return counts;
}

// Who the participant is comes from the shared reading; which member of this
// company that address belongs to is the importer's own business.
function resolveParticipant(participant: DeviceCalendarParticipant): ResolvedParticipant | undefined {
	const match = matchParticipant(participant, devicePeople, emailByPersonID);
	if (!match) return undefined;
	const memberID = memberByEmail.get(match.email);
	return memberID ? { memberID, email: match.email, by: match.by } : undefined;
}
