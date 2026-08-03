// Counts what a company has on the central plane, so what is still on a device
// is a subtraction rather than a guess.
//   bun run web/scripts/show-migration.ts

import { controlPlane } from '../src/lib/server/control-plane';

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

async function count(table: string, filter?: (query: ReturnType<typeof client.from>) => unknown): Promise<number> {
	const query = client.from(table).select('*', { count: 'exact', head: true });
	const { count: total, error } = await (filter ? (filter(query as never) as typeof query) : query);
	if (error) throw new Error(`${table}: ${error.message}`);
	return total ?? 0;
}

const rows: [string, number][] = [
	['company', await count('company')],
	['member', await count('member')],
	['team', await count('team')],
	['attendance', await count('attendance')],
	['leave', await count('leave')],
	['task (work)', await count('task', (query) => (query as never as { eq: (a: string, b: boolean) => unknown }).eq('is_event', false))],
	['task (events)', await count('task', (query) => (query as never as { eq: (a: string, b: boolean) => unknown }).eq('is_event', true))],
	['task_participant', await count('task_participant')],
	['credential', await count('credential')],
	['agent', await count('agent')],
];

for (const [name, total] of rows) console.log(`${name.padEnd(18)} ${total}`);

const attendance = await client.from('attendance').select('occurred_at').order('occurred_at').limit(1);
const latest = await client.from('attendance').select('occurred_at').order('occurred_at', { ascending: false }).limit(1);
if (attendance.data?.[0] && latest.data?.[0]) {
	console.log(`\nattendance spans ${attendance.data[0].occurred_at.slice(0, 10)} → ${latest.data[0].occurred_at.slice(0, 10)}`);
}
