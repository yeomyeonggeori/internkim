//   bun run web/scripts/show-members.ts

import { controlPlane } from '../src/lib/server/control-plane';

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? '',
});

const { data, error } = await client
	.from('member')
	.select('name, email, job_title, status, team:team_id (name)')
	.order('email');
if (error) throw new Error(error.message);

for (const member of data) {
	const team = (member.team as { name?: string } | null)?.name ?? '';
	console.log(`${(member.name ?? '(no name)').padEnd(12)} ${member.email?.padEnd(24)} ${(member.job_title ?? '').padEnd(16)} ${team.padEnd(12)} ${member.status}`);
}
