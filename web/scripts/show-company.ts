// Prints what a company looks like in Supabase. Reads credentials from .env.
//   bun run web/scripts/show-company.ts

import { controlPlane } from '../src/lib/server/control-plane';

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data: companies, error: companyError } = await client
	.from('company')
	.select('id, name, slug, country, locale, timezone, work_locations, leave_days');
if (companyError) throw new Error(companyError.message);

for (const company of companies ?? []) {
	console.log(`company ${company.name} (${company.slug}) ${company.country}/${company.locale} ${company.timezone}`);
	console.log(`  work locations: ${(company.work_locations ?? []).join(', ') || 'none'}`);

	const { data: members } = await client
		.from('member')
		.select('email, is_admin, status, user_id, joined_at')
		.eq('company_id', company.id)
		.order('is_admin', { ascending: false });
	for (const member of members ?? []) {
		const bound = member.user_id ? 'bound' : 'unbound';
		console.log(
			`  ${(member.email ?? '').padEnd(24)} ${member.is_admin ? 'admin ' : 'member'} ${member.status.padEnd(8)} ${bound}`,
		);
	}

	const { count } = await client.from('attendance').select('*', { count: 'exact', head: true });
	const { count: leaveCount } = await client.from('leave').select('*', { count: 'exact', head: true });
	console.log(`  attendance rows: ${count ?? 0}   leave rows: ${leaveCount ?? 0}`);
	const { data: locations } = await client.from('attendance').select('location');
	const tally = new Map<string, number>();
	for (const row of locations ?? []) tally.set(row.location ?? '(none)', (tally.get(row.location ?? '(none)') ?? 0) + 1);
	console.log(`  locations: ${[...tally].map(([name, n]) => `${name} ${n}`).join(', ')}`);

	const { data: leaves } = await client
		.from('leave')
		.select('kind, is_paid, is_deducted, days, status, starts_at, ends_at, note')
		.order('starts_at');
	const dayIn = (instant: string) =>
		new Date(instant).toLocaleDateString('sv-SE', { timeZone: company.timezone });
	for (const leave of leaves ?? []) {
		console.log(
			`  leave ${leave.kind} paid=${leave.is_paid} deducted=${leave.is_deducted} days=${leave.days} ` +
				`${dayIn(leave.starts_at)}~${dayIn(leave.ends_at)} ${leave.note ?? ''}`,
		);
	}
}

const { data: accounts } = await client.auth.admin.listUsers();
console.log(`auth accounts: ${accounts.users.length}`);
for (const account of accounts.users) {
	console.log(`  ${(account.email ?? '').padEnd(24)} ${account.user_metadata?.full_name ?? ''}`);
}
