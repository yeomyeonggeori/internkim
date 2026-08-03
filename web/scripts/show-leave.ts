// Shows what leave a company has on record.
//   bun run web/scripts/show-leave.ts

import { controlPlane } from '../src/lib/server/control-plane';

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data, error } = await client
	.from('leave')
	.select('kind, status, days, is_paid, starts_at, ends_at, member:member_id (name)')
	.order('starts_at');
if (error) throw new Error(error.message);

console.log(`${data.length} rows`);
for (const row of data.slice(0, 20)) {
	const who = (row.member as { name?: string } | null)?.name ?? '';
	console.log(`${who.padEnd(8)} ${row.kind.padEnd(10)} ${String(row.days).padEnd(5)} ${row.status.padEnd(10)} ${row.starts_at.slice(0, 10)} → ${row.ends_at.slice(0, 10)}`);
}
