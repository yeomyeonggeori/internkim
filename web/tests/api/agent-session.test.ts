import { afterAll, beforeAll, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import {
	addMember,
	controlPlane,
	issueAgentKey,
	provisionCompany,
	sessionForPlatformIdentity,
} from '../../src/lib/server/control-plane';
import { connectMessengerAccount } from '../../src/lib/server/member-credential';
import { claimABoxWithItsSession, releaseTheClaimedBox, type ClaimedBoxWithSession } from '../support/a-claimed-box';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

const networkHookTimeout = 60_000;

const credentials = { projectURL, publishableKey, serviceRoleKey, signingKey };

const client = controlPlane(credentials);
const stamp = Date.now();
let ourCompanyID = '';
let theirCompanyID = '';
let ourBox: ClaimedBoxWithSession;
let speakerID = '';

async function withAccount(memberID: string): Promise<void> {
	const { data } = await client.from('member').select('email').eq('id', memberID).single();
	const { data: account } = await client.auth.admin.createUser({
		email: data!.email!,
		email_confirm: true,
	});
	await client.from('member').update({ user_id: account.user!.id }).eq('id', memberID);
}

beforeAll(async () => {
	const ours = await provisionCompany(
		client,
		{ name: 'Ours', slug: `agent-ours-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`agent-ours-${stamp}-admin@example.test`,
	);
	ourCompanyID = ours.companyID;
	ourBox = await claimABoxWithItsSession(credentials, ourCompanyID);

	speakerID = await addMember(client, ourCompanyID, `agent-speaker-${stamp}@example.test`);
	await withAccount(speakerID);
	await connectMessengerAccount(client, ourCompanyID, {
		memberID: speakerID,
		platform: 'buzz',
		kind: 'buzz-secret',
		externalID: `speaker-${stamp}`,
		name: 'Speaker',
		secret: `speaker-secret-${stamp}`,
	});

	const theirs = await provisionCompany(
		client,
		{ name: 'Theirs', slug: `agent-theirs-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`agent-theirs-${stamp}-admin@example.test`,
	);
	theirCompanyID = theirs.companyID;
	const outsiderID = await addMember(client, theirCompanyID, `agent-outsider-${stamp}@example.test`);
	await withAccount(outsiderID);
	await connectMessengerAccount(client, theirCompanyID, {
		memberID: outsiderID,
		platform: 'buzz',
		kind: 'buzz-secret',
		externalID: `outsider-${stamp}`,
		name: 'Outsider',
		secret: `outsider-secret-${stamp}`,
	});
}, networkHookTimeout);

afterAll(async () => {
	for (const companyID of [ourCompanyID, theirCompanyID].filter(Boolean)) {
		const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
		await client.from('company').delete().eq('id', companyID);
		for (const member of members ?? []) {
			if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
		}
	}
}, networkHookTimeout);

test('an agent acts as the member who spoke on its own platform', async () => {
	const session = await sessionForPlatformIdentity(credentials, ourBox.session.accessToken, 'buzz', `speaker-${stamp}`);
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

test('an agent cannot act for somebody at another company', async () => {
	await expect(
		sessionForPlatformIdentity(credentials, ourBox.session.accessToken, 'buzz', `outsider-${stamp}`),
	).rejects.toThrow('no member here');
});

test('an identity nobody claims is refused', async () => {
	await expect(
		sessionForPlatformIdentity(credentials, ourBox.session.accessToken, 'buzz', 'nobody-at-all'),
	).rejects.toThrow('no member');
});

test('a made-up key is refused', async () => {
	await expect(
		sessionForPlatformIdentity(credentials, 'not-a-real-key', 'buzz', `speaker-${stamp}`),
	).rejects.toThrow('belongs to no company computer');
});

test('an agent key is no company computer', async () => {
	const { apiKey } = await issueAgentKey(client, ourCompanyID, `ticket-${stamp}`);

	await expect(sessionForPlatformIdentity(credentials, apiKey, 'buzz', `speaker-${stamp}`)).rejects.toThrow(
		'belongs to no company computer',
	);
});

test('a second box acts for the company, and the first stops once the company lets it go', async () => {
	const spare = await claimABoxWithItsSession(credentials, ourCompanyID);

	const bySpare = await sessionForPlatformIdentity(credentials, spare.session.accessToken, 'buzz', `speaker-${stamp}`);
	const byFirst = await sessionForPlatformIdentity(credentials, ourBox.session.accessToken, 'buzz', `speaker-${stamp}`);
	expect(bySpare.memberID).toBe(speakerID);
	expect(byFirst.memberID).toBe(speakerID);

	await releaseTheClaimedBox(client, ourCompanyID, ourBox);
	await expect(
		sessionForPlatformIdentity(credentials, ourBox.session.accessToken, 'buzz', `speaker-${stamp}`),
	).rejects.toThrow('belongs to no company computer');
	const stillWorks = await sessionForPlatformIdentity(credentials, spare.session.accessToken, 'buzz', `speaker-${stamp}`);
	expect(stillWorks.memberID).toBe(speakerID);
});

test('a connection file key can be rotated more than once', async () => {
	const first = await issueAgentKey(client, ourCompanyID, 'rotating', { replaceStanding: true });
	const second = await issueAgentKey(client, ourCompanyID, 'rotating', { replaceStanding: true });
	const third = await issueAgentKey(client, ourCompanyID, 'rotating', { replaceStanding: true });

	expect(second.apiKey).not.toBe(first.apiKey);
	expect(third.apiKey).not.toBe(second.apiKey);
	expect(third.agentID).toBe(first.agentID);

	const { data } = await client
		.from('agent')
		.select('id, revoked_at')
		.eq('company_id', ourCompanyID)
		.eq('name', 'rotating');
	expect(data).toHaveLength(1);
	expect(data![0].revoked_at).toBeNull();
});

test('two live agents cannot share a name', async () => {
	await issueAgentKey(client, ourCompanyID, 'only-one');
	expect(issueAgentKey(client, ourCompanyID, 'only-one')).rejects.toThrow(/duplicate key/);
});
