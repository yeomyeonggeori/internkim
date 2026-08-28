//   bun run web/scripts/import-flow-state.ts --file <flow-state.json> --people <organization-people.json> --company <uuid> [--apply]

import { controlPlane } from '../src/lib/server/control-plane';
import { readAllRows } from './read-all-rows';
import { emailByPersonIDOf, matchParticipant, type DevicePerson, type EventCalendar } from '../../host/relay/calendar-event-as-task';
import { taskAsTask, peopleOnTask, titleOfTask, type DeviceTask } from '../../host/relay/flow-task-as-task';

type DeviceState = {
	tasks: DeviceTask[];
	definitions: { categories: string[]; categoryColors?: Record<string, string>; types: string[] };
};

type LiveTask = { id: string; title: string; ends_at: string | null; calendar: unknown };

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const file = argument('file');
const peopleFile = argument('people');
const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!file || !peopleFile || !companyID) {
	throw new Error('pass --file <flow-state.json> --people <organization-people.json> --company <uuid>');
}

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? ''
});

const state = JSON.parse(await Bun.file(file).text()) as DeviceState;
const devicePeople = (JSON.parse(await Bun.file(peopleFile).text()) as { records?: DevicePerson[] }).records ?? [];
const emailByPersonID = emailByPersonIDOf(devicePeople);

const { data: company, error: companyError } = await client
	.from('company')
	.select('timezone')
	.eq('id', companyID)
	.single<{ timezone: string | null }>();
if (companyError) throw new Error(companyError.message);
const timeZone = company.timezone || 'UTC';

const memberRows = await readAllRows<{ id: string; email: string | null }>((from, to) =>
	client
		.from('member')
		.select('id, email')
		.eq('company_id', companyID)
		.order('id', { ascending: true })
		.range(from, to)
);
const memberByEmail = new Map(memberRows.map((member) => [(member.email ?? '').toLowerCase(), member.id]));

const liveTasks = await readAllRows<LiveTask>((from, to) =>
	client
		.from('task')
		.select('id, title, ends_at, calendar')
		.eq('company_id', companyID)
		.eq('is_event', false)
		.order('id', { ascending: true })
		.range(from, to)
);

const idByExternalID = new Map<string, string>();
for (const task of liveTasks) {
	for (const mirror of (task.calendar as EventCalendar | null)?.mirrors ?? []) {
		idByExternalID.set(mirror.externalID, task.id);
	}
}
const claimedIDs = new Set<string>();

let adopted = 0;
let updated = 0;
let inserted = 0;
const skippedTitles: string[] = [];
const peopleMatchingNoMember = new Set<string>();
const matchedByName = new Set<string>();

for (const task of state.tasks) {
	const title = titleOfTask(task);
	const written = peopleOnTask(task);

	const memberIDs = new Set<string>();
	for (const person of written) {
		const match = matchParticipant(person, devicePeople, emailByPersonID);
		const memberID = match && memberByEmail.get(match.email);
		if (!memberID) {
			peopleMatchingNoMember.add(person.name?.trim() || (person.personID ?? '(이름 없음)'));
			continue;
		}
		memberIDs.add(memberID);
		if (match.by !== 'personID') matchedByName.add(`${person.name?.trim()} → ${match.email}`);
	}

	if (written.length > 0 && memberIDs.size === 0) {
		skippedTitles.push(title);
		continue;
	}

	const asTask = taskAsTask(task, timeZone);
	const fields = {
		company_id: companyID,
		title: asTask.title,
		status: asTask.status,
		note: asTask.note,
		business: asTask.business,
		type: asTask.type,
		size: asTask.size,
		starts_at: asTask.startsAt,
		ends_at: asTask.endsAt,
		is_whole_day: asTask.isWholeDay,
		calendar: asTask.calendar
	};

	const mirrored = task.id ? idByExternalID.get(task.id) : undefined;
	const known = mirrored ?? adoptTaskImportedBeforeMirrors(title, asTask.endsAt);
	if (known) claimedIDs.add(known);
	if (known && !mirrored) adopted += 1;

	if (!shouldApply) {
		known ? (updated += 1) : (inserted += 1);
		continue;
	}

	const record = known
		? await client.from('task').update(fields).eq('id', known).select('id').single()
		: await client.from('task').insert(fields).select('id').single();
	if (record.error) throw new Error(`${title}: ${record.error.message}`);
	known ? (updated += 1) : (inserted += 1);

	if (memberIDs.size === 0) continue;
	const { error: participantError } = await client
		.from('task_participant')
		.upsert([...memberIDs].map((memberID) => ({ task_id: record.data.id, member_id: memberID })));
	if (participantError) throw new Error(`${title} participants: ${participantError.message}`);
}

if (shouldApply) {
	const vocabulary = {
		businesses: state.definitions.categories.map((name) => ({
			name,
			...(state.definitions.categoryColors?.[name] ? { color: state.definitions.categoryColors[name] } : {})
		})),
		types: state.definitions.types.map((name) => ({ name }))
	};
	const { error } = await client.from('company').update({ task_vocabulary: vocabulary }).eq('id', companyID);
	if (error) throw new Error(`vocabulary: ${error.message}`);
	console.log(`vocabulary: ${vocabulary.businesses.length} businesses, ${vocabulary.types.length} types`);
}

console.log(`${shouldApply ? 'wrote' : 'would write'}: ${updated} updated, ${inserted} inserted, ${skippedTitles.length} skipped`);
if (adopted) console.log(`adopted ${adopted} tasks that an earlier import left without a mirror`);
if (matchedByName.size) console.log(`matched by name rather than by id: ${[...matchedByName].join(', ')}`);
if (peopleMatchingNoMember.size) console.log(`on a task but no member holds their address: ${[...peopleMatchingNoMember].join(', ')}`);
if (skippedTitles.length) console.log(`skipped work belonging to nobody in this company: ${skippedTitles.join(', ')}`);

// The board was imported before a device id rode along, so those rows carry no
// mirror. They are adopted by the identity that import used, title and the day
// the work ends, and stamped with a mirror as they are updated.
//
// A board holds two tasks under one title and day often enough — two "새 일정"
// on the same date — and nothing here can tell which copy is which. Claiming
// one apiece pairs them off, where insisting on a unique match would write a
// duplicate for every one of them.
function adoptTaskImportedBeforeMirrors(title: string, endsAt: string | null): string | undefined {
	const endDay = endsAt?.slice(0, 10) ?? null;
	const same = liveTasks
		.filter((live) => !claimedIDs.has(live.id) && !hasMirror(live))
		.filter((live) => live.title.trim() === title && (live.ends_at?.slice(0, 10) ?? null) === endDay);
	return same.sort((one, other) => one.id.localeCompare(other.id))[0]?.id;
}

function hasMirror(live: LiveTask): boolean {
	return ((live.calendar as EventCalendar | null)?.mirrors ?? []).length > 0;
}
