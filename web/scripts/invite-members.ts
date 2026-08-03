// Sends invitations. Without --email everyone still pending is invited, so a first
// rollout is one command and a rerun only reaches whoever has not been asked yet.
//   bun run web/scripts/invite-members.ts --company <uuid> [--email one@example.com] [--redirect <url>] [--apply]

import { controlPlane } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const companyID = argument('company');
const onlyEmail = argument('email');
const redirectTo = argument('redirect');
const shouldApply = process.argv.includes('--apply');
if (!companyID) throw new Error('pass --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data: members, error } = await client
	.from('member')
	.select('id, email, status')
	.eq('company_id', companyID)
	.eq('status', 'pending');
if (error) throw new Error(error.message);

const targets = (members ?? []).filter((member) => member.email && (!onlyEmail || member.email === onlyEmail));
console.log(`pending members: ${members?.length ?? 0}, inviting: ${targets.length}`);
for (const member of targets) console.log(`  ${member.email}`);

if (!shouldApply) {
	console.log('\ndry run — pass --apply to send');
	process.exit(0);
}

for (const member of targets) {
	const { error: inviteError } = await client.auth.admin.inviteUserByEmail(member.email!, {
		redirectTo,
	});
	if (inviteError) {
		console.log(`  ${member.email} failed: ${inviteError.message}`);
		continue;
	}
	const { error: statusError } = await client
		.from('member')
		.update({ status: 'invited' })
		.eq('id', member.id);
	if (statusError) throw new Error(statusError.message);
	console.log(`  ${member.email} invited`);
}
