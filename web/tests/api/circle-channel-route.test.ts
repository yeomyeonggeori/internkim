import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { z } from 'zod';
import { addMember, controlPlane, issueAgentKey, provisionCompany } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { GET: readCircleChannels, PUT: recordCircleChannel } = await import(
	'../../src/routes/api/agent/circle-channel/+server'
);

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const stamp = Date.now();
const environment = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
};
const slug = `circle-channel-${stamp}`;
const inCircleEmail = `${slug}-in@example.test`;
const leftEmail = `${slug}-left@example.test`;

const answeredCircles = z.object({
	circles: z.array(
		z.strictObject({
			circleID: z.string(),
			channelName: z.string().nullable(),
			channelID: z.string().nullable(),
			memberEmails: z.array(z.string())
		})
	)
});

let companyID = '';
let agentKey = '';

function asTheHost(init: RequestInit = {}): { request: Request; platform: { env: typeof environment } } {
	const request = new Request('https://intern.example.test/api/agent/circle-channel', {
		...init,
		headers: { authorization: `Bearer ${agentKey}`, 'content-type': 'application/json' }
	});
	return { request, platform: { env: environment } };
}

async function circlesTheHostReads() {
	const response = await readCircleChannels(asTheHost() as unknown as Parameters<typeof readCircleChannels>[0]);
	expect(response.status).toBe(200);
	return answeredCircles.parse(await response.json()).circles;
}

async function hostRecords(body: Record<string, unknown>): Promise<number> {
	const event = asTheHost({ method: 'PUT', body: JSON.stringify(body) });
	try {
		const response = await recordCircleChannel(event as unknown as Parameters<typeof recordCircleChannel>[0]);
		return response.status;
	} catch (refusal) {
		return (refusal as { status: number }).status;
	}
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Circle Channel', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	agentKey = (await issueAgentKey(client, companyID, 'circle-channel')).apiKey;
	const inCircle = await addMember(client, companyID, inCircleEmail);
	const left = await addMember(client, companyID, leftEmail);
	await client.from('member').update({ status: 'active' }).eq('id', inCircle);
	await client.from('member').update({ status: 'withdrawn' }).eq('id', left);
	await client.from('circle_member').insert([
		{ company_id: companyID, circle_id: 'finance', member_id: inCircle },
		{ company_id: companyID, circle_id: 'finance', member_id: left }
	]);
}, networkHookTimeout);

afterAll(async () => {
	await client.from('company').delete().eq('id', companyID);
}, networkHookTimeout);

describe('the circle channels a host keeps', () => {
	test('names no circle until one is given a channel', async () => {
		expect(await circlesTheHostReads()).toEqual([]);
	});

	test('names a wanted channel with the active people of its circle', async () => {
		await client.from('circle').update({ channel_name: '재무' }).eq('company_id', companyID).eq('id', 'finance');

		expect(await circlesTheHostReads()).toEqual([
			{ circleID: 'finance', channelName: '재무', channelID: null, memberEmails: [inCircleEmail] }
		]);
	});

	test('records the channel the host opened, and its taking down', async () => {
		expect(await hostRecords({ circleID: 'finance', channelID: '00000000-0000-4000-8000-0000000000c1' })).toBe(200);
		expect((await circlesTheHostReads())[0]?.channelID).toBe('00000000-0000-4000-8000-0000000000c1');

		await client.from('circle').update({ channel_name: null }).eq('company_id', companyID).eq('id', 'finance');
		expect((await circlesTheHostReads())[0]).toMatchObject({ channelName: null, channelID: '00000000-0000-4000-8000-0000000000c1' });

		expect(await hostRecords({ circleID: 'finance', channelID: null })).toBe(200);
		expect(await circlesTheHostReads()).toEqual([]);
	});

	test('refuses a circle the company does not have', async () => {
		expect(await hostRecords({ circleID: 'no-such-circle', channelID: null })).toBe(404);
	});
});
