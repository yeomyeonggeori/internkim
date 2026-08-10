//   bun run web/scripts/check-broadcast-refusal.ts --url https://<project>.supabase.co

import { createClient, type RealtimeChannel, type SupabaseClient } from '@supabase/supabase-js';
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

const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

async function clientFor(accessToken: string): Promise<SupabaseClient> {
	const client = createClient(projectURL, publishableKey, {
		auth: { autoRefreshToken: false, persistSession: false }
	});
	await client.realtime.setAuth(accessToken);
	return client;
}

function joined(channel: RealtimeChannel): Promise<void> {
	return new Promise<void>((resolve, reject) => {
		const giveUp = setTimeout(() => reject(new Error(`${channel.topic} never joined`)), 10_000);
		channel.subscribe((status) => {
			if (status !== 'SUBSCRIBED') return;
			clearTimeout(giveUp);
			resolve();
		});
	});
}

async function refused(sending: Promise<unknown>): Promise<boolean> {
	try {
		await sending;
		return false;
	} catch {
		return true;
	}
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
	{ name: 'Refusal check', slug: `refusal-check-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
	`first-${stamp}@example.test`
);
let failed = false;
try {
	const secondID = await addMember(admin, company.companyID, `second-${stamp}@example.test`);
	await signUp(`first-${stamp}@example.test`, company.adminMemberID);
	await signUp(`second-${stamp}@example.test`, secondID);
	const first = await sessionForMember({ projectURL, serviceRoleKey }, company.adminMemberID);
	const second = await sessionForMember({ projectURL, serviceRoleKey }, secondID);

	const client = await clientFor(first.accessToken);
	const heard: string[] = [];
	const own = client.channel(`member:${first.memberID}`, { config: { private: true } });
	own.on('broadcast', { event: 'answer' }, ({ payload }) => heard.push(String(payload?.mark ?? '')));
	await joined(own);

	await own.httpSend('answer', { mark: 'delivered' });
	await new Promise((resolve) => setTimeout(resolve, 3_000));

	const colleague = client.channel(`member:${second.memberID}`, { config: { private: true } });
	const httpSendRefused = await refused(colleague.httpSend('answer', { mark: 'stolen' }));
	const sendReported = await colleague.send({
		type: 'broadcast',
		event: 'answer',
		payload: { mark: 'stolen' }
	});

	const findings = [
		['a channel it joined can httpSend into its own topic', heard.includes('delivered')],
		['and it stays joined afterwards, so replies keep flowing', own.state === 'joined'],
		["httpSend into a colleague's topic is refused, loudly", httpSendRefused],
		['send() reports ok for that same refusal, which is why the relay does not use it', sendReported === 'ok']
	] as const;

	for (const [what, held] of findings) console.log(`${held ? 'ok  ' : 'FAIL'} ${what}`);
	failed = findings.some(([, held]) => !held);

	await client.removeAllChannels();
	client.realtime.disconnect();
} finally {
	await admin.from('company').delete().eq('id', company.companyID);
	const accounts = await admin.auth.admin.listUsers();
	for (const account of accounts.data.users) {
		if (account.email?.includes(stamp)) await admin.auth.admin.deleteUser(account.id);
	}
}

if (failed) process.exit(1);
