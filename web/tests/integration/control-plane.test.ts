import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import {
	addMember,
	controlPlane,
	inviteMember,
	linkCredential,
	memberOfPlatformIdentity,
	provisionCompany,
} from '../../src/lib/server/control-plane';

// Runs against a local Supabase stack:
//   supabase start && SUPABASE_URL=... SUPABASE_SERVICE_ROLE_KEY=... bun test tests/integration
const projectURL = process.env.SUPABASE_URL ?? '';
const serviceRoleKey = process.env.SUPABASE_SERVICE_ROLE_KEY ?? '';
const canReachSupabase = Boolean(projectURL && serviceRoleKey);

const client = canReachSupabase ? controlPlane({ projectURL, serviceRoleKey }) : null;
const slug = `control-plane-test-${Date.now()}`;
const adminEmail = `${slug}-admin@example.test`;
const colleagueEmail = `${slug}-colleague@example.test`;
let companyID = '';

beforeAll(async () => {
	if (!client) return;
	const provisioned = await provisionCompany(
		client,
		{
			name: 'Control Plane Test',
			slug,
			country: 'KR',
			locale: 'ko',
			timezone: 'Asia/Seoul',
			workLocations: ['Headquarters'],
		},
		adminEmail,
	);
	companyID = provisioned.companyID;
});

afterAll(async () => {
	if (!client || !companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
});

if (!canReachSupabase) {
	test('supabase is not reachable, so provisioning is not exercised', () => {
		expect(canReachSupabase).toBe(false);
	});
}

if (canReachSupabase)
	describe('provisioning a company', () => {
	test('creates the company with an admin member', async () => {
		const { data } = await client!
			.from('member')
			.select('email, is_admin, status')
			.eq('company_id', companyID);

		expect(data).toHaveLength(1);
		expect(data![0]).toMatchObject({ email: adminEmail, is_admin: true, status: 'pending' });
	});

	test('a member exists before anyone has an account', async () => {
		const memberID = await addMember(client!, companyID, colleagueEmail);
		const { data } = await client!.from('member').select('user_id, status').eq('id', memberID).single();

		expect(data!.user_id).toBeNull();
		expect(data!.status).toBe('pending');
	});

	test('inviting binds the member to the account it creates', async () => {
		const memberID = await addMember(client!, companyID, colleagueEmail);
		await inviteMember(client!, memberID);
		const { data } = await client!.from('member').select('user_id, status').eq('id', memberID).single();

		expect(data!.user_id).not.toBeNull();
		expect(data!.status).toBe('invited');
	});

	test('a platform identity resolves back to its member', async () => {
		const memberID = await addMember(client!, companyID, colleagueEmail);
		await linkCredential(client!, memberID, 'buzz', `pubkey-${slug}`);

		expect(await memberOfPlatformIdentity(client!, 'buzz', `pubkey-${slug}`)).toBe(memberID);
		expect(await memberOfPlatformIdentity(client!, 'buzz', 'nobody')).toBeNull();
	});
});
