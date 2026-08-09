//   bun run web/scripts/import-leave.ts --file <attendance.json> [--apply]

import { controlPlane } from '../src/lib/server/control-plane';

type DeviceAbsence = {
	id: string;
	rangeID?: string;
	email: string;
	kind: string;
	startDate?: string;
	endDate?: string;
	date: string;
	reason?: string;
	createdAt: string;
	canceledAt?: string;
};

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const file = argument('file');
const shouldApply = process.argv.includes('--apply');
if (!file) throw new Error('pass --file <attendance.json>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const document = JSON.parse(await Bun.file(file).text()) as { absences?: DeviceAbsence[] };

const ranges = new Map<string, DeviceAbsence>();
for (const absence of document.absences ?? []) {
	if (absence.canceledAt) continue;
	ranges.set(absence.rangeID ?? absence.id, absence);
}

const { data: members, error: memberError } = await client.from('member').select('id, email');
if (memberError) throw new Error(memberError.message);
const memberByEmail = new Map((members ?? []).map((member) => [member.email ?? '', member.id]));

const { data: existing, error: leaveError } = await client.from('leave').select('member_id, starts_at, ends_at');
if (leaveError) throw new Error(leaveError.message);
const here = new Set((existing ?? []).map((row) => `${row.member_id}|${dayOf(row.starts_at)}|${dayOf(row.ends_at)}`));

let written = 0;
let already = 0;
const unknownPeople = new Set<string>();

for (const absence of ranges.values()) {
	const memberID = memberByEmail.get(absence.email);
	if (!memberID) {
		unknownPeople.add(absence.email);
		continue;
	}
	const firstDay = absence.startDate ?? absence.date;
	const lastDay = absence.endDate ?? firstDay;
	const startsAt = `${firstDay}T00:00:00+09:00`;
	const endsAt = `${shiftedDay(lastDay, 1)}T00:00:00+09:00`;

	if (here.has(`${memberID}|${dayOf(startsAt)}|${dayOf(endsAt)}`)) {
		already += 1;
		continue;
	}
	if (!shouldApply) {
		written += 1;
		continue;
	}

	const { error } = await client.from('leave').insert({
		member_id: memberID,
		kind: absence.kind === 'leave' ? '연차' : absence.kind,
		is_paid: true,
		days: inclusiveDays(firstDay, lastDay),
		status: 'approved',
		starts_at: startsAt,
		ends_at: endsAt,
		note: absence.reason ?? null,
	});
	if (error) throw new Error(`${absence.email} ${firstDay}: ${error.message}`);
	written += 1;
}

console.log(`${shouldApply ? 'wrote' : 'would write'}: ${written}, already there: ${already}, of ${ranges.size} on the device`);
if (unknownPeople.size) console.log(`addresses that match no member: ${[...unknownPeople].join(', ')}`);

function dayOf(instant: string): string {
	return new Date(instant).toISOString().slice(0, 10);
}

function shiftedDay(day: string, days: number): string {
	const moved = new Date(`${day}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + days);
	return moved.toISOString().slice(0, 10);
}

function inclusiveDays(firstDay: string, lastDay: string): number {
	const from = new Date(`${firstDay}T00:00:00Z`).getTime();
	const to = new Date(`${lastDay}T00:00:00Z`).getTime();
	return Math.max(1, Math.round((to - from) / 86_400_000) + 1);
}
