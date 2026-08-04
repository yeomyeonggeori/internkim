// Brings a device's work across from what its own web app serves, so no SSH and
// no database file are needed. Matches a task by title and end day, the same
// identity the earlier import used, so rerunning updates instead of duplicating.
//   bun run web/scripts/import-flow-state.ts --file <flow-state.json> --company <uuid> [--apply]

import { controlPlane } from '../src/lib/server/control-plane';

type DeviceTask = {
	content: string;
	goal: string;
	business: string;
	type: string;
	size: string;
	status: string;
	startDate?: string;
	endDate?: string;
	participantNames: string[];
	ownerName: string;
	requestReason?: string;
};

type DeviceState = {
	tasks: DeviceTask[];
	definitions: { categories: string[]; categoryColors?: Record<string, string>; types: string[] };
};

const statusOfDevice: Record<string, string> = {
	요청: 'todo',
	예정: 'todo',
	진행: 'in_progress',
	완료: 'done',
	일시정지: 'paused',
	기각: 'cancelled',
	중단: 'cancelled'
};

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const file = argument('file');
const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!file || !companyID) throw new Error('pass --file <flow-state.json> --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? ''
});

const state = JSON.parse(await Bun.file(file).text()) as DeviceState;

const { data: members, error: memberError } = await client
	.from('member')
	.select('id, name')
	.eq('company_id', companyID);
if (memberError) throw new Error(memberError.message);
const memberByName = new Map((members ?? []).map((member) => [member.name ?? '', member.id]));

const { data: existing, error: taskError } = await client
	.from('task')
	.select('id, title, ends_at, starts_at')
	.eq('company_id', companyID)
	.eq('is_event', false);
if (taskError) throw new Error(taskError.message);
const idByIdentity = new Map((existing ?? []).map((task) => [identityOf(task.title, dayOf(task.ends_at)), task.id]));

let updated = 0;
let inserted = 0;
let unmatchedNames = new Set<string>();

for (const task of state.tasks) {
	const title = task.content?.trim() || '(제목 없음)';
	const endDay = dayPartOf(task.endDate);
	const startDay = dayPartOf(task.startDate);
	const fields = {
		company_id: companyID,
		title,
		status: statusOfDevice[task.status?.trim()] ?? 'todo',
		note: noteOf(task),
		business: task.business?.trim() || null,
		type: task.type?.trim() || null,
		size: task.size?.trim() || null,
		// A task with only an end started on the day it was due; a task with only a
		// start has no end yet.
		starts_at: dayStart(startDay ?? endDay),
		ends_at: dayEnd(endDay),
		is_whole_day: Boolean(startDay && endDay)
	};

	// An earlier import filled a missing end in with the start. Those rows are
	// found by that invented end, then corrected rather than duplicated.
	const known =
		idByIdentity.get(identityOf(title, endDay)) ??
		(!endDay && startDay ? idByIdentity.get(identityOf(title, startDay)) : undefined);
	if (!shouldApply) {
		known ? (updated += 1) : (inserted += 1);
		continue;
	}

	const written = known
		? await client.from('task').update(fields).eq('id', known).select('id').single()
		: await client.from('task').insert(fields).select('id').single();
	if (written.error) throw new Error(`${title}: ${written.error.message}`);
	known ? (updated += 1) : (inserted += 1);

	const names = new Set([task.ownerName, ...(task.participantNames ?? [])].map((name) => name?.trim()).filter(Boolean));
	const memberIDs = [...names].map((name) => {
		const memberID = memberByName.get(name!);
		if (!memberID) unmatchedNames.add(name!);
		return memberID;
	}).filter((memberID): memberID is string => Boolean(memberID));
	if (memberIDs.length === 0) continue;
	const { error: participantError } = await client
		.from('task_participant')
		.upsert(memberIDs.map((memberID) => ({ task_id: written.data.id, member_id: memberID })));
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

console.log(`${shouldApply ? 'wrote' : 'would write'}: ${updated} updated, ${inserted} inserted`);
if (unmatchedNames.size) console.log(`names that match no member: ${[...unmatchedNames].join(', ')}`);

function noteOf(task: DeviceTask): string | null {
	const parts = [task.goal?.trim() && `목표: ${task.goal.trim()}`, task.requestReason?.trim()].filter(Boolean);
	return parts.join('\n') || null;
}

function identityOf(title: string, endDay: string | null): string {
	return `${title.trim()}@${endDay ?? ''}`;
}

function dayOf(instant: string | null): string | null {
	return instant ? instant.slice(0, 10) : null;
}

function dayStart(day: string | null): string | null {
	return day ? `${day}T00:00:00+09:00` : null;
}

function dayEnd(day: string | null): string | null {
	return day ? `${day}T23:59:00+09:00` : null;
}

function dayPartOf(value: string | undefined): string | null {
	const day = (value ?? '').slice(0, 10);
	return /^\d{4}-\d{2}-\d{2}$/.test(day) ? day : null;
}
