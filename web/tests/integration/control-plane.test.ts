import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import {
	addMember,
	controlPlane,
	inviteMember,
	provisionCompany,
} from '../../src/lib/server/control-plane';
import {
	connectMessengerAccount,
	membersOfCompanyByExternalID,
} from '../../src/lib/server/member-credential';
import { projectURL, serviceRoleKey } from './supabase-environment';

const networkHookTimeout = 60_000;

const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `control-plane-test-${Date.now()}`;
const adminEmail = `${slug}-admin@example.test`;
const colleagueEmail = `${slug}-colleague@example.test`;
let companyID = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{
			name: 'Control Plane Test',
			slug,
			country: 'KR',
			locale: 'ko',
			timezone: 'Asia/Seoul',
			workLocations: [{ name: 'Headquarters', color: '#0ea5e9' }],
		},
		adminEmail,
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

describe('provisioning a company', () => {
	test('creates the company with an admin member', async () => {
		const { data } = await client
			.from('member')
			.select('email, is_admin, status')
			.eq('company_id', companyID);

		expect(data).toHaveLength(1);
		expect(data![0]).toMatchObject({ email: adminEmail, is_admin: true, status: 'pending' });
	});

	test('a member exists before anyone has an account', async () => {
		const memberID = await addMember(client, companyID, colleagueEmail);
		const { data } = await client.from('member').select('user_id, status').eq('id', memberID).single();

		expect(data!.user_id).toBeNull();
		expect(data!.status).toBe('pending');
	});

	test('inviting binds the member to the account it creates', async () => {
		const memberID = await addMember(client, companyID, colleagueEmail);
		await inviteMember(client, memberID);
		const { data } = await client.from('member').select('user_id, status').eq('id', memberID).single();

		expect(data!.user_id).not.toBeNull();
		expect(data!.status).toBe('invited');
	});

	test('a platform identity resolves back to its member', async () => {
		const memberID = await addMember(client, companyID, colleagueEmail);
		await connectMessengerAccount(client, companyID, {
			memberID,
			platform: 'buzz',
			kind: 'buzz-secret',
			externalID: `pubkey-${slug}`,
			name: 'Colleague',
			secret: `colleague-secret-${slug}`,
		});

		const byExternalID = await membersOfCompanyByExternalID(client, companyID, 'buzz');

		expect(byExternalID.get(`pubkey-${slug}`)).toBe(memberID);
		expect(byExternalID.get('nobody')).toBeUndefined();
	});
});
