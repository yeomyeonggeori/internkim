// Prints a sign-in link instead of mailing one. Supabase's built-in mailer only
// reaches addresses on the project team, so until custom SMTP is configured this is
// how somebody gets in — hand them the link over whatever they already use.
//   bun run web/scripts/sign-in-link.ts --company <uuid> [--email one@example.com] [--redirect http://localhost:5599/clock]

import { controlPlane } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const companyID = argument('company');
const onlyEmail = argument('email');
const redirectTo = argument('redirect');
if (!companyID) throw new Error('pass --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data: members, error } = await client
	.from('member')
	.select('id, email, status')
	.eq('company_id', companyID)
	.not('email', 'is', null);
if (error) throw new Error(error.message);

const targets = (members ?? []).filter((member) => !onlyEmail || member.email === onlyEmail);
if (targets.length === 0) throw new Error('nobody matched');

for (const member of targets) {
	const { data, error: linkError } = await client.auth.admin.generateLink({
		type: 'magiclink',
		email: member.email!,
		options: redirectTo ? { redirectTo } : undefined,
	});
	if (linkError) {
		console.log(`${member.email}: ${linkError.message}`);
		continue;
	}
	console.log(`\n${member.email}`);
	console.log(data.properties.action_link);
}

console.log('\nEach link signs that person in once. They expire, so send them when they are wanted.');
