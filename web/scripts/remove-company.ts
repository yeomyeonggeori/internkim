import { controlPlane } from '../src/lib/server/control-plane';

// Integration tests build a company and delete it in afterAll. A hook that times
// out leaves it behind, so this removes one by the slug it was given. Slugs are
// named rather than matched, because a pattern that reaches one company reaches
// the company people work in.

const slugs = process.argv.slice(2);
if (slugs.length === 0) throw new Error('name the company slugs to remove');

const projectURL = process.env.SUPABASE_URL ?? '';
const serviceRoleKey = process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '';
if (!projectURL || !serviceRoleKey) throw new Error('needs SUPABASE_URL and SUPABASE_SECRET_KEY');

const client = controlPlane({ projectURL, serviceRoleKey });

for (const slug of slugs) {
	const { data: company, error } = await client
		.from('company')
		.select('id, name')
		.eq('slug', slug)
		.maybeSingle<{ id: string; name: string }>();
	if (error) throw new Error(`${slug}: ${error.message}`);
	if (!company) {
		console.log(`${slug}: already gone`);
		continue;
	}

	const { data: members } = await client
		.from('member')
		.select('user_id, email')
		.eq('company_id', company.id)
		.returns<{ user_id: string | null; email: string }[]>();

	const removed = await client.from('company').delete().eq('id', company.id);
	if (removed.error) throw new Error(`${slug}: ${removed.error.message}`);

	for (const member of members ?? []) {
		if (!member.user_id) continue;
		const { error: accountError } = await client.auth.admin.deleteUser(member.user_id);
		if (accountError) console.log(`${member.email}: account left behind, ${accountError.message}`);
	}
	console.log(`${slug}: removed ${company.name} and ${members?.length ?? 0} members`);
}
