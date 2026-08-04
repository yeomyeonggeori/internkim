// Brings a device's attendance across from what its own web app serves.
//   bun run web/scripts/import-attendance-events.ts --file <attendance.json> [--apply]
//
// Records go in as they happened, oldest first. The table's clock-in/clock-out
// rule guards new input, so a row the past violated is reported rather than
// reshaped into something the device never recorded.

import { controlPlane } from '../src/lib/server/control-plane';

type DeviceEvent = {
	email: string;
	kind: 'clock_in' | 'clock_out';
	occurredAt: string;
	locationName?: string;
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

const document = JSON.parse(await Bun.file(file).text()) as { events?: DeviceEvent[] };
const deviceEvents = (document.events ?? []).filter((event) => !event.canceledAt);

const { data: members, error: memberError } = await client.from('member').select('id, email');
if (memberError) throw new Error(memberError.message);
const memberByEmail = new Map((members ?? []).map((member) => [member.email ?? '', member.id]));
const emailByMember = new Map((members ?? []).map((member) => [member.id, member.email ?? '']));

const { data: rows, error: rowError } = await client.from('attendance').select('member_id, kind, occurred_at');
if (rowError) throw new Error(rowError.message);
const here = new Set((rows ?? []).map((row) => keyOf(emailByMember.get(row.member_id) ?? '', row.kind, row.occurred_at)));

const seen = new Set<string>();
const missing = deviceEvents
	.filter((event) => {
		const key = keyOf(event.email, event.kind, event.occurredAt);
		if (here.has(key) || seen.has(key)) return false;
		seen.add(key);
		return true;
	})
	.sort((left, right) => left.occurredAt.localeCompare(right.occurredAt));

let written = 0;
const rejected: { event: DeviceEvent; reason: string }[] = [];
const unknownPeople = new Set<string>();

for (const event of missing) {
	const memberID = memberByEmail.get(event.email);
	if (!memberID) {
		unknownPeople.add(event.email);
		continue;
	}
	if (!shouldApply) {
		written += 1;
		continue;
	}
	const { error } = await client.from('attendance').insert({
		member_id: memberID,
		kind: event.kind,
		location: event.kind === 'clock_in' ? (event.locationName || null) : null,
		occurred_at: event.occurredAt,
	});
	if (error) {
		rejected.push({ event, reason: error.message });
		continue;
	}
	written += 1;
}

console.log(`${shouldApply ? 'wrote' : 'would write'}: ${written} of ${missing.length} missing`);
if (unknownPeople.size) console.log(`addresses that match no member: ${[...unknownPeople].join(', ')}`);
for (const { event, reason } of rejected) {
	console.log(`  refused ${event.email} ${event.kind} ${event.occurredAt}: ${reason.split('\n')[0]}`);
}

function keyOf(email: string, kind: string, occurredAt: string): string {
	return `${email}|${kind}|${new Date(occurredAt).toISOString()}`;
}
