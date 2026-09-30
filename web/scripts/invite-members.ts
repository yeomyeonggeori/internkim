//   bun run web/scripts/invite-members.ts --company <uuid> [--email one@example.com] [--apply]

import { controlPlane, inviteMember } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const companyID = argument('company');
const onlyEmail = argument('email');
const shouldApply = process.argv.includes('--apply');
if (!companyID) throw new Error('pass --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? '',
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

console.log('\nHand these over yourself — they are not mailed:\n');
for (const member of targets) {
	const invitation = await inviteMember(client, member.id);
	console.log(`  ${invitation.email.padEnd(28)} ${invitation.temporaryPassword}`);
}
console.log('\nEveryone should change theirs after signing in.');
