//   bun run web/scripts/import-calendar-events.ts --file <calendar-events.json> --company <uuid> [--apply]

import { controlPlane } from '../src/lib/server/control-plane';

type DeviceParticipant = { personID?: string; name?: string };

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
	people?: string[];
	participants?: DeviceParticipant[];
	reminderLeadHours?: number;
	createdByEmail?: string;
	remoteSource?: string;
	remoteHref?: string;
};

type Member = { id: string; name: string | null; email: string | null };

type LiveEvent = { id: string; title: string; starts_at: string; calendar: unknown };

type Mirror = { source: string; externalID: string; href?: string };

type EventCalendar = { timeZone?: string; color?: string; mirrors: Mirror[] };

const deviceSource = 'internkim-device';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const file = argument('file');
const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!file || !companyID) throw new Error('pass --file <calendar-events.json> --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? ''
});

const events = (JSON.parse(await Bun.file(file).text()) as { events?: DeviceEvent[] }).events ?? [];

const { data: memberRows, error: memberError } = await client
	.from('member')
	.select('id, name, email')
	.eq('company_id', companyID);
if (memberError) throw new Error(memberError.message);
const members = (memberRows ?? []) as Member[];
const memberByEmail = new Map(members.map((member) => [(member.email ?? '').toLowerCase(), member.id]));

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

const deviceTitleCounts = countTitles(events.map((event) => event.title));
const claimedIDs = new Set<string>();

let adopted = 0;
let updated = 0;
let inserted = 0;
const skippedTitles: string[] = [];
const namesMatchingNobody = new Set<string>();
const creatorsMatchingNobody = new Set<string>();
const refusedByTheRecord: string[] = [];

for (const event of events) {
	const title = event.title?.trim() || '(제목 없음)';

	if (!(Date.parse(event.endISO) >= Date.parse(event.startISO))) {
		refusedByTheRecord.push(`${title}: ends ${event.endISO} before it starts ${event.startISO}`);
		continue;
	}

	const attendeeNames = attendeeNamesOf(event);
	const attendeeIDs = new Set<string>();
	for (const name of attendeeNames) {
		const memberID = resolveMember(name);
		memberID ? attendeeIDs.add(memberID) : namesMatchingNobody.add(name);
	}

	const createdByEmail = (event.createdByEmail ?? '').toLowerCase();
	const requesterID = memberByEmail.get(createdByEmail);
	if (createdByEmail && !requesterID) creatorsMatchingNobody.add(createdByEmail);

	if (!requesterID && attendeeIDs.size === 0 && attendeeNames.size > 0) {
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
	if (!shouldApply) {
		known ? (updated += 1) : (inserted += 1);
		continue;
	}

	const written = known
		? await client.from('task').update(fields).eq('id', known).select('id').single()
		: await client.from('task').insert(fields).select('id').single();
	if (written.error) throw new Error(`${title}: ${written.error.message}`);
	known ? (updated += 1) : (inserted += 1);

	if (attendeeIDs.size === 0) continue;
	const { error: participantError } = await client
		.from('task_participant')
		.upsert([...attendeeIDs].map((memberID) => ({ task_id: written.data.id, member_id: memberID })));
	if (participantError) throw new Error(`${title} participants: ${participantError.message}`);
}

console.log(`${shouldApply ? 'wrote' : 'would write'}: ${updated} updated, ${inserted} inserted, ${skippedTitles.length} skipped`);
if (adopted) console.log(`adopted ${adopted} events that an earlier import left without a mirror`);
if (skippedTitles.length) console.log(`skipped, nobody in this company asked for them or is on them: ${skippedTitles.join(', ')}`);
if (namesMatchingNobody.size) console.log(`named on an event but not a member, left off: ${[...namesMatchingNobody].join(', ')}`);
if (creatorsMatchingNobody.size) console.log(`created by an address no member holds, kept without a requester: ${[...creatorsMatchingNobody].join(', ')}`);
for (const refusal of refusedByTheRecord) console.log(`refused by the record: ${refusal}`);

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

// The device stores an attendee as free text, so a colleague is written both in
// full and by given name alone. A single member whose name ends with what was
// written is that person; two would be a guess, so it stays unresolved.
function resolveMember(writtenName: string): string | undefined {
	const exact = members.find((member) => member.name === writtenName);
	if (exact) return exact.id;
	if (writtenName.length < 2) return undefined;
	const ending = members.filter((member) => (member.name ?? '').endsWith(writtenName));
	return ending.length === 1 ? ending[0].id : undefined;
}

function attendeeNamesOf(event: DeviceEvent): Set<string> {
	const written = [...(event.people ?? []), ...(event.participants ?? []).map((participant) => participant.name ?? '')];
	return new Set(written.map((name) => name.trim()).filter(Boolean));
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
