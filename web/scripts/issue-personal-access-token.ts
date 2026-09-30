//   bun run web/scripts/issue-personal-access-token.ts --company <uuid> --email <member email> --name <what to call it> [--permission read|write|delete] [--quiet]

import { controlPlane, issuePersonalAccessToken } from '../src/lib/server/control-plane';
import { memberOfCompanyByEmail } from '../src/lib/server/member-credential';
import { publicAPIPermissionOf } from '../src/lib/public-api-permission';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const companyID = argument('company');
const email = argument('email');
const name = argument('name') ?? 'first';
const permission = publicAPIPermissionOf(argument('permission') ?? 'write');
if (!companyID || !email) throw new Error('pass --company <uuid> --email <member email>');
if (!permission) throw new Error('--permission is one of read, write, delete');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? '',
});

const memberID = await memberOfCompanyByEmail(client, companyID, email);
if (!memberID) throw new Error(`${email} is not a member of company ${companyID}`);

const token = await issuePersonalAccessToken(client, memberID, name, permission);

if (process.argv.includes('--quiet')) {
	console.log(token);
} else {
	console.log(`\nA ${permission} token for ${email}, shown once:\n`);
	console.log(`  ${token}\n`);
}
