// Says which of a device's attendance records never made it across.
//   bun run web/scripts/compare-attendance.ts --file <attendance.json>

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
if (!file) throw new Error('pass --file <attendance.json>');

const document = JSON.parse(await Bun.file(file).text()) as { events?: DeviceEvent[] };
const deviceEvents = document.events ?? [];
const live = deviceEvents.filter((event) => !event.canceledAt);

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data: members, error: memberError } = await client.from('member').select('id, email');
if (memberError) throw new Error(memberError.message);
const emailByMember = new Map((members ?? []).map((member) => [member.id, member.email ?? '']));

const { data: rows, error: rowError } = await client.from('attendance').select('member_id, kind, occurred_at');
if (rowError) throw new Error(rowError.message);
const here = new Set((rows ?? []).map((row) => keyOf(emailByMember.get(row.member_id) ?? '', row.kind, row.occurred_at)));

const missing = live.filter((event) => !here.has(keyOf(event.email, event.kind, event.occurredAt)));

console.log(`device: ${live.length} live, ${deviceEvents.length - live.length} cancelled`);
console.log(`plane: ${rows?.length ?? 0}`);
console.log(`missing: ${missing.length}`);
for (const event of missing) {
	console.log(`  ${event.email.padEnd(24)} ${event.kind.padEnd(10)} ${event.occurredAt} ${event.locationName ?? ''}`);
}

function keyOf(email: string, kind: string, occurredAt: string): string {
	return `${email}|${kind}|${new Date(occurredAt).toISOString()}`;
}
