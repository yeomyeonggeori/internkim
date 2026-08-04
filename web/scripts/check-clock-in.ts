//   bun run web/scripts/check-clock-in.ts --email <address>

import { controlPlane } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const email = argument('email');
if (!email) throw new Error('pass --email <address>');

const member = await client.from('member').select('id, name').eq('email', email).single<{ id: string; name: string }>();
if (member.error) throw new Error(member.error.message);
const memberID = member.data.id;

const inserted = await client
	.from('attendance')
	.insert({ member_id: memberID, kind: 'clock_in', occurred_at: new Date().toISOString() })
	.select('id, location')
	.single<{ id: string; location: string | null }>();

if (inserted.error) {
	console.log(`refused: ${inserted.error.message}`);
	process.exit(1);
}

console.log(`accepted, location resolved to ${inserted.data.location}`);
await client.from('attendance').delete().eq('id', inserted.data.id);
console.log('removed the probe row');
