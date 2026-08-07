//   bun run web/scripts/check-topic-isolation.ts --url http://127.0.0.1:54321 --key <service role> --publishable <publishable key>

import { createClient } from '@supabase/supabase-js';
import { provisionCompany, sessionForMember, addMember } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const projectURL = argument('url') ?? process.env.SUPABASE_URL ?? '';
const serviceRoleKey = argument('key') ?? process.env.SUPABASE_SECRET_KEY ?? '';
const publishableKey = argument('publishable') ?? process.env.SUPABASE_PUBLISHABLE_KEY ?? '';
if (!projectURL || !serviceRoleKey || !publishableKey) {
	throw new Error('pass --url, --key and --publishable');
}

const credentials = { projectURL, serviceRoleKey };
const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

async function canJoin(accessToken: string, topic: string): Promise<boolean> {
	const client = createClient(projectURL, publishableKey, {
		auth: { autoRefreshToken: false, persistSession: false }
	});
	await client.realtime.setAuth(accessToken);
	const channel = client.channel(topic, { config: { private: true } });
	const joined = await new Promise<boolean>((resolve) => {
		const giveUp = setTimeout(() => resolve(false), 8_000);
		channel.subscribe((status) => {
			if (status === 'SUBSCRIBED') {
				clearTimeout(giveUp);
				resolve(true);
			}
			if (status === 'CHANNEL_ERROR' || status === 'TIMED_OUT' || status === 'CLOSED') {
				clearTimeout(giveUp);
				resolve(false);
			}
		});
	});
	await client.removeAllChannels();
	client.realtime.disconnect();
	return joined;
}

async function signUp(email: string, memberID: string): Promise<void> {
	const account = await admin.auth.admin.createUser({ email, email_confirm: true });
	if (account.error) throw new Error(`${email}: ${account.error.message}`);
	const linked = await admin.from('member').update({ user_id: account.data.user.id }).eq('id', memberID);
	if (linked.error) throw new Error(`${email}: ${linked.error.message}`);
}

const stamp = crypto.randomUUID().slice(0, 8);
const company = await provisionCompany(
	admin,
	{ name: 'Topic check', slug: `topic-check-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
	`first-${stamp}@example.test`
);
let failed = false;
try {
	const secondID = await addMember(admin, company.companyID, `second-${stamp}@example.test`);
	await signUp(`first-${stamp}@example.test`, company.adminMemberID);
	await signUp(`second-${stamp}@example.test`, secondID);
	const first = await sessionForMember(credentials, company.adminMemberID);
	const second = await sessionForMember(credentials, secondID);

	const findings = [
		['a member joins their own answer topic', await canJoin(first.accessToken, `member:${first.memberID}`)],
		["and not a colleague's", !(await canJoin(first.accessToken, `member:${second.memberID}`))],
		[
			'a member joins the company presence topic',
			await canJoin(first.accessToken, `company:${company.companyID}`)
		],
		[
			'and cannot read the calls anyone makes',
			!(await canJoin(first.accessToken, `company:${company.companyID}:call`))
		]
	] as const;

	for (const [what, held] of findings) console.log(`${held ? 'ok  ' : 'FAIL'} ${what}`);
	failed = findings.some(([, held]) => !held);
} finally {
	await admin.from('company').delete().eq('id', company.companyID);
	const accounts = await admin.auth.admin.listUsers();
	for (const account of accounts.data.users) {
		if (account.email?.includes(stamp)) await admin.auth.admin.deleteUser(account.id);
	}
}

if (failed) process.exit(1);
