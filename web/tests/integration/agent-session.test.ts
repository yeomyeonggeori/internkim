import { afterAll, beforeAll, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import {
	addMember,
	controlPlane,
	issueHostSecret,
	linkCredential,
	provisionCompany,
	sessionForPlatformIdentity,
} from '../../src/lib/server/control-plane';

const projectURL = process.env.SUPABASE_URL ?? '';
const serviceRoleKey = process.env.SUPABASE_SERVICE_ROLE_KEY ?? '';
const publishableKey = process.env.SUPABASE_PUBLISHABLE_KEY ?? '';
const canReachSupabase = Boolean(projectURL && serviceRoleKey && publishableKey);
const credentials = { projectURL, serviceRoleKey };

const client = canReachSupabase ? controlPlane(credentials) : null;
const stamp = Date.now();
let ourCompanyID = '';
let theirCompanyID = '';
let ourHostSecret = '';
let speakerID = '';

async function withAccount(memberID: string): Promise<void> {
	const { data } = await client!.from('member').select('email').eq('id', memberID).single();
	const { data: account } = await client!.auth.admin.createUser({
		email: data!.email!,
		email_confirm: true,
	});
	await client!.from('member').update({ user_id: account.user!.id }).eq('id', memberID);
}

beforeAll(async () => {
	if (!client) return;
	const ours = await provisionCompany(
		client,
		{ name: 'Ours', slug: `agent-ours-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`agent-ours-${stamp}-admin@example.test`,
	);
	ourCompanyID = ours.companyID;
	ourHostSecret = (await issueHostSecret(client, ourCompanyID)).secret;

	speakerID = await addMember(client, ourCompanyID, `agent-speaker-${stamp}@example.test`);
	await withAccount(speakerID);
	await linkCredential(client, speakerID, 'buzz', `speaker-${stamp}`);

	const theirs = await provisionCompany(
		client,
		{ name: 'Theirs', slug: `agent-theirs-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`agent-theirs-${stamp}-admin@example.test`,
	);
	theirCompanyID = theirs.companyID;
	const outsiderID = await addMember(client, theirCompanyID, `agent-outsider-${stamp}@example.test`);
	await withAccount(outsiderID);
	await linkCredential(client, outsiderID, 'buzz', `outsider-${stamp}`);
});

afterAll(async () => {
	if (!client) return;
	for (const companyID of [ourCompanyID, theirCompanyID].filter(Boolean)) {
		const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
		await client.from('company').delete().eq('id', companyID);
		for (const member of members ?? []) {
			if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
		}
	}
});

if (!canReachSupabase) {
	test('supabase is not reachable, so the agent path is not exercised', () => {
		expect(canReachSupabase).toBe(false);
	});
}

if (canReachSupabase) {
	test('a host acts as the member who spoke on its own platform', async () => {
		const session = await sessionForPlatformIdentity(credentials, ourHostSecret, 'buzz', `speaker-${stamp}`);
		expect(session.memberID).toBe(speakerID);

		const asMember = createClient(projectURL, publishableKey, {
			global: { headers: { Authorization: `Bearer ${session.accessToken}` } },
			auth: { persistSession: false, autoRefreshToken: false },
		});
		const { error: writeError } = await asMember
			.from('attendance')
			.insert({ member_id: speakerID, kind: 'clock_in' });
		expect(writeError).toBeNull();
	});

	test('a host cannot act for somebody at another company', async () => {
		await expect(
			sessionForPlatformIdentity(credentials, ourHostSecret, 'buzz', `outsider-${stamp}`),
		).rejects.toThrow('another company');
	});

	test('an identity nobody claims is refused', async () => {
		await expect(
			sessionForPlatformIdentity(credentials, ourHostSecret, 'buzz', 'nobody-at-all'),
		).rejects.toThrow('no member');
	});

	test('a made-up host secret is refused', async () => {
		await expect(
			sessionForPlatformIdentity(credentials, 'not-a-real-secret', 'buzz', `speaker-${stamp}`),
		).rejects.toThrow('no company');
	});

	test('the secret is not stored in a usable form', async () => {
		const { data } = await client!
			.from('company')
			.select('host_secret_hash')
			.eq('id', ourCompanyID)
			.single();

		expect(data!.host_secret_hash).not.toBe(ourHostSecret);
		expect(data!.host_secret_hash).toHaveLength(64);
	});
}
