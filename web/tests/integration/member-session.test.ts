import { afterAll, beforeAll, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { importJWK, jwtVerify } from 'jose';
import {
	addMember,
	controlPlane,
	provisionCompany,
	sessionForMember,
} from '../../src/lib/server/control-plane';
import {
	connectMessengerAccount,
	membersOfCompanyByExternalID,
} from '../../src/lib/server/member-credential';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

const networkHookTimeout = 60_000;


const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `session-test-${Date.now()}`;
let companyID = '';
let speakerID = '';
let colleagueID = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Session Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`,
	);
	companyID = provisioned.companyID;

	speakerID = await addMember(client, companyID, `${slug}-speaker@example.test`);
	colleagueID = await addMember(client, companyID, `${slug}-colleague@example.test`);
	for (const memberID of [speakerID, colleagueID]) {
		const { data } = await client
			.from('member')
			.select('email')
			.eq('id', memberID)
			.single();
		const { data: account } = await client.auth.admin.createUser({
			email: data!.email!,
			email_confirm: true,
		});
		await client.from('member').update({ user_id: account.user!.id }).eq('id', memberID);
	}
	await connectMessengerAccount(client, companyID, {
		memberID: speakerID,
		platform: 'buzz',
		kind: 'buzz-secret',
		externalID: `pubkey-${slug}`,
		name: 'Speaker',
		secret: `speaker-secret-${slug}`,
	});
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

test('a platform identity resolves to the member who owns it', async () => {
	const byExternalID = await membersOfCompanyByExternalID(client, companyID, 'buzz');

	expect(byExternalID.get(`pubkey-${slug}`)).toBe(speakerID);
});

test('the session acts as that member and nobody else', async () => {
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, speakerID);
	const asMember = createClient(projectURL, publishableKey, {
		global: { headers: { Authorization: `Bearer ${session.accessToken}` } },
		auth: { persistSession: false, autoRefreshToken: false },
	});

	const { error: ownError } = await asMember
		.from('attendance')
		.insert({ member_id: speakerID, kind: 'clock_in' });
	expect(ownError).toBeNull();

	const { error: colleagueError } = await asMember
		.from('attendance')
		.insert({ member_id: colleagueID, kind: 'clock_in' });
	expect(colleagueError).not.toBeNull();
});

test('the token is the plane\'s own word about the member and opens no auth session', async () => {
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, speakerID);
	const { payload } = await jwtVerify(session.accessToken, await importJWK(JSON.parse(signingKey), 'HS256'));
	const { data: member } = await client.from('member').select('user_id').eq('id', speakerID).single();

	expect(payload.sub).toBe(member!.user_id);
	expect(payload.role).toBe('authenticated');
	expect(payload.session_id).toBeUndefined();
});

test('the session is short lived', async () => {
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, speakerID);
	const secondsLeft = session.expiresAt - Math.floor(Date.now() / 1000);

	expect(secondsLeft > 0).toBe(true);
	expect(secondsLeft <= 60 * 60 * 24).toBe(true);
});

test('somebody who has left cannot be acted for', async () => {
	const updateResult = await client
		.from('member')
		.update({ status: 'departed' })
		.eq('id', colleagueID)
		.select();
	expect(updateResult.error).toBeNull();
	expect(updateResult.data).toHaveLength(1);

	const { data: after } = await client
		.from('member')
		.select('status')
		.eq('id', colleagueID)
		.single();
	expect(after!.status).toBe('departed');

	await expect(sessionForMember({ projectURL, serviceRoleKey, signingKey }, colleagueID)).rejects.toThrow('has left');
});
