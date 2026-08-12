//   bun run web/scripts/import-calendar-events.ts --file <calendar-events.json> --people <organization-people.json> --company <uuid> [--apply]

import { controlPlane } from '../src/lib/server/control-plane';

type DeviceParticipant = { personID?: string; name?: string };

type DevicePerson = { userID?: string; name?: string; handle?: string; email?: string; image?: string };

type ResolvedParticipant = { memberID: string; email: string; by: 'personID' | 'nameOrHandle' | 'givenName' };

type DeviceEvent = {
	id: string;
	uid: string;
	title: string;
	description?: string;
	location?: string;
	startISO: string;
	endISO: string;
	timeZone?: string;
	isAllDay?: boolean;
	color?: string;
	participants?: DeviceParticipant[];
	reminderLeadHours?: number;
	createdByEmail?: string;
	remoteSource?: string;
	remoteHref?: string;
};

type LiveEvent = { id: string; title: string; starts_at: string; calendar: unknown };

type EventFields = { title: string; starts_at: string; calendar: EventCalendar };

type Mirror = { source: string; externalID: string; href?: string };

type EventCalendar = { timeZone?: string; color?: string; mirrors: Mirror[] };

const deviceSource = 'internkim-device';

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
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? ''
});

const events = (JSON.parse(await Bun.file(file).text()) as { events?: DeviceEvent[] }).events ?? [];
const devicePeople = (JSON.parse(await Bun.file(peopleFile).text()) as { records?: DevicePerson[] }).records ?? [];

const { data: memberRows, error: memberError } = await client
	.from('member')
	.select('id, email')
	.eq('company_id', companyID);
if (memberError) throw new Error(memberError.message);
const memberByEmail = new Map((memberRows ?? []).map((member) => [(member.email ?? '').toLowerCase(), member.id]));

const emailByPersonID = personDirectoryOf(devicePeople);

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
const matchedByGivenName = new Set<string>();
const creatorsMatchingNobody = new Set<string>();
const refusedByTheRecord: string[] = [];
const heldTwiceByTheDevice: string[] = [];

for (const event of events) {
	const title = event.title?.trim() || '(제목 없음)';

	if (!(Date.parse(event.endISO) >= Date.parse(event.startISO))) {
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
		if (found.by === 'givenName') matchedByGivenName.add(`${participant.name?.trim()} → ${found.email}`);
	}

	const createdByEmail = (event.createdByEmail ?? '').toLowerCase();
	const requesterID = memberByEmail.get(createdByEmail);
	if (createdByEmail && !requesterID) creatorsMatchingNobody.add(createdByEmail);

	if (!requesterID && attendeeIDs.size === 0 && writtenParticipants.length > 0) {
		skippedTitles.push(title);
		continue;
	}

	const fields = {
		company_id: companyID,
		title,
		is_event: true,
		is_whole_day: Boolean(event.isAllDay),
		starts_at: event.startISO,
		ends_at: event.endISO,
		note: event.description?.trim() || null,
		location: event.location?.trim() ? { name: event.location.trim() } : null,
		notify_minutes_before: reminderMinutesOf(event.reminderLeadHours),
		requester_id: requesterID ?? null,
		calendar: calendarOf(event)
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
		: await client.from('task').insert(fields).select('id').single();
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
if (matchedByGivenName.size) console.log(`matched by a given name only one member bears: ${[...matchedByGivenName].join(', ')}`);
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
function adoptEventImportedBeforeMirrors(event: DeviceEvent): string | undefined {
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

// A participant is a person the device identified, so their id decides who they
// are. Two shapes carry that id: the account uuid, and the shorter one the
// calendar uses, which this export prints only inside the participant image path.
function personDirectoryOf(people: DevicePerson[]): Map<string, string> {
	const emailByPersonID = new Map<string, string>();
	for (const person of people) {
		const email = (person.email ?? '').toLowerCase();
		if (!email) continue;
		if (person.userID) emailByPersonID.set(person.userID, email);
		const calendarID = /\/participants\/([0-9a-f]+)\/image/.exec(person.image ?? '')?.[1];
		if (calendarID) emailByPersonID.set(calendarID, email);
	}
	return emailByPersonID;
}

// Older rows carry a name where a newer one carries an id, so a name still has to
// resolve. A full name or handle is matched outright; a given name only when one
// member bears it, because two would be a guess. The rest is nobody we know.
function resolveParticipant(participant: DeviceParticipant): ResolvedParticipant | undefined {
	const personID = (participant.personID ?? '').trim();
	if (personID) {
		const email = emailByPersonID.get(personID);
		const memberID = memberByEmail.get(email ?? '');
		return email && memberID ? { memberID, email, by: 'personID' } : undefined;
	}

	const name = participant.name?.trim() ?? '';
	if (name.length < 2) return undefined;
	const named = devicePeople.find((person) => person.name === name || person.handle === name);
	if (named) return memberOf(named.email, 'nameOrHandle');

	const bearing = devicePeople.filter((person) => (person.name ?? '').endsWith(name));
	return bearing.length === 1 ? memberOf(bearing[0].email, 'givenName') : undefined;
}

function memberOf(email: string | undefined, by: ResolvedParticipant['by']): ResolvedParticipant | undefined {
	const address = (email ?? '').toLowerCase();
	const memberID = memberByEmail.get(address);
	return memberID ? { memberID, email: address, by } : undefined;
}

function reminderMinutesOf(leadHours: number | undefined): number | null {
	if (typeof leadHours !== 'number' || !Number.isFinite(leadHours) || leadHours <= 0) return null;
	return Math.round(leadHours * 60);
}

function calendarOf(event: DeviceEvent): EventCalendar {
	const externalID = event.uid || event.id;
	const mirrors: Mirror[] = [{ source: deviceSource, externalID }];
	if (event.remoteSource && event.remoteHref) {
		mirrors.push({ source: event.remoteSource, externalID, href: event.remoteHref });
	}
	return {
		...(event.timeZone ? { timeZone: event.timeZone } : {}),
		...(event.color ? { color: event.color } : {}),
		mirrors
	};
}
