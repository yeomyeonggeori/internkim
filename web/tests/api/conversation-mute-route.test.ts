import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, controlPlane, issueAgentKey, provisionCompany } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { POST: muteForMember } = await import('../../src/routes/api/agent/conversation-mute/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const stamp = Date.now();
const environment = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
};
const slug = `conversation-mute-${stamp}`;
const otherSlug = `conversation-mute-other-${stamp}`;

let companyID = '';
let otherCompanyID = '';
let agentKey = '';
let seatedMemberID = '';
let otherCompanyMemberID = '';

async function statusOfMute(body: Record<string, unknown>, bearer = agentKey): Promise<number> {
	const request = new Request('https://intern.example.test/api/agent/conversation-mute', {
		method: 'POST',
		headers: { authorization: `Bearer ${bearer}`, 'content-type': 'application/json' },
		body: JSON.stringify(body)
	});
	const event = { request, platform: { env: environment } };
	try {
		const response = await muteForMember(event as unknown as Parameters<typeof muteForMember>[0]);
		return response.status;
	} catch (refusal) {
		return (refusal as { status: number }).status;
	}
}

async function mutedChannelsOf(memberID: string): Promise<unknown> {
	const member = await client.from('member').select('preferences').eq('id', memberID).single<{ preferences: Record<string, unknown> }>();
	if (member.error) throw new Error(member.error.message);
	return member.data.preferences.mutedChannels ?? [];
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Conversation Mute', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	agentKey = (await issueAgentKey(client, companyID, 'conversation-mute')).apiKey;
	seatedMemberID = await addMember(client, companyID, `${slug}-seated@example.test`);
	await client.from('member').update({ preferences: { mutedChannels: ['dm-own-choice'] } }).eq('id', seatedMemberID);

	const other = await provisionCompany(
		client,
		{ name: 'Conversation Mute Other', slug: otherSlug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${otherSlug}-admin@example.test`
	);
	otherCompanyID = other.companyID;
	otherCompanyMemberID = await addMember(client, otherCompanyID, `${otherSlug}-member@example.test`);
}, networkHookTimeout);

afterAll(async () => {
	await client.from('company').delete().in('id', [companyID, otherCompanyID]);
}, networkHookTimeout);

describe('the company computer mutes a channel for a member it seated', () => {
	test('adds the channel once beside what the member muted themselves', async () => {
		expect(await statusOfMute({ memberID: seatedMemberID, conversationID: 'channel-seated' })).toBe(200);
		expect(await statusOfMute({ memberID: seatedMemberID, conversationID: 'channel-seated' })).toBe(200);

		expect(await mutedChannelsOf(seatedMemberID)).toEqual(['dm-own-choice', 'channel-seated']);
	});

	test('refuses a member of another company and leaves them told', async () => {
		expect(await statusOfMute({ memberID: otherCompanyMemberID, conversationID: 'channel-seated' })).toBe(403);

		expect(await mutedChannelsOf(otherCompanyMemberID)).toEqual([]);
	});

	test('refuses a caller who does not hold the company computer credential', async () => {
		expect(await statusOfMute({ memberID: seatedMemberID, conversationID: 'channel-other' }, 'ik_not-a-host-credential')).toBe(403);

		expect(await mutedChannelsOf(seatedMemberID)).toEqual(['dm-own-choice', 'channel-seated']);
	});

	test('refuses a mute that names no conversation', async () => {
		expect(await statusOfMute({ memberID: seatedMemberID })).toBe(400);
	});
});
