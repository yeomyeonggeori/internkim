import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import {
	agentOfKey,
	controlPlane,
	issueAgentKey,
	provisionCompany,
} from '../../src/lib/server/control-plane';
import { projectURL, serviceRoleKey } from './supabase-environment';

const networkHookTimeout = 60_000;

const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `agent-key-test-${Date.now()}`;
let companyID = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{
			name: 'Agent Key Test',
			slug,
			country: 'KR',
			locale: 'ko',
			timezone: 'Asia/Seoul',
			workLocations: [{ name: 'Headquarters', color: '#0ea5e9' }],
		},
		`${slug}-admin@example.test`,
	);
	companyID = provisioned.companyID;
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

describe('an agent key belongs to the name it was issued under', () => {
	test('two fleets of one company each keep the key they were given', async () => {
		const first = await issueAgentKey(client, companyID, 'internkim-e2e-first', {
			replaceStanding: true,
		});
		const second = await issueAgentKey(client, companyID, 'internkim-e2e-second', {
			replaceStanding: true,
		});

		expect(await agentOfKey(client, first.apiKey)).toMatchObject({ companyID });
		expect(await agentOfKey(client, second.apiKey)).toMatchObject({ companyID });
	});

	test('issuing again under one name retires the key that name held', async () => {
		const standing = await issueAgentKey(client, companyID, 'internkim-e2e-first', {
			replaceStanding: true,
		});
		const replacement = await issueAgentKey(client, companyID, 'internkim-e2e-first', {
			replaceStanding: true,
		});

		expect(await agentOfKey(client, standing.apiKey)).toBeNull();
		expect(await agentOfKey(client, replacement.apiKey)).toMatchObject({ companyID });
	});
});
