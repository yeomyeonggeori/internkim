// Sets a password on an account so somebody can sign in without waiting for mail.
// For trying the thing out — a real rollout invites people instead.
//   bun run web/scripts/set-password.ts --email you@example.com --password '...'

import { controlPlane } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const email = argument('email');
const password = argument('password');
if (!email || !password) throw new Error("pass --email <address> --password '<value>'");

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data: accounts, error: listError } = await client.auth.admin.listUsers();
if (listError) throw new Error(listError.message);
const account = accounts.users.find((user) => user.email === email);
if (!account) throw new Error(`no account for ${email}`);

const { error } = await client.auth.admin.updateUserById(account.id, {
	password,
	email_confirm: true,
});
if (error) throw new Error(error.message);

const { error: statusError } = await client
	.from('member')
	.update({ status: 'active' })
	.eq('user_id', account.id);
if (statusError) throw new Error(statusError.message);

console.log(`${email} can now sign in, and their member is active`);
