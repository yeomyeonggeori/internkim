import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import {
	addMember,
	AddressBelongsToAnotherCompany,
	adminCallerOf,
	AlreadyAMember,
	asMember,
	closeSignInOfMembersWhoLeft,
	controlPlane,
	foundCompany,
	hostAddressOf,
	inviteMember,
	provisionCompany,
	removeMember,
	resetMemberPassword,
	settleSignInOfMember,
} from '../../src/lib/server/control-plane';
import {
	connectMessengerAccount,
	memberOfCompanyByEmail,
	membersOfCompanyByExternalID,
} from '../../src/lib/server/member-credential';
import { recordTokenFor } from '../../src/lib/server/record-token';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

const networkHookTimeout = 60_000;

const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `control-plane-test-${Date.now()}`;
const adminEmail = `${slug}-admin@example.test`;
const colleagueEmail = `${slug}-colleague@example.test`;
const otherSlug = `${slug}-other`;
const otherAdminEmail = `${otherSlug}-admin@example.test`;
const lateEmail = `${slug}-late@example.test`;
const fillerCount = 55;
let companyID = '';
let otherCompanyID = '';
const fillerAccountIDs: string[] = [];
const strayAccountIDs: string[] = [];

function companyInput(companySlug: string) {
	return { name: 'Control Plane Test', slug: companySlug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' };
}

async function memberOf(email: string) {
	const { data, error } = await client
		.from('member')
		.select('id, company_id, is_admin, status, user_id')
		.eq('email', email)
		.single();
	if (error) throw new Error(error.message);
	return data;
}

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
	otherCompanyID = (await provisionCompany(client, companyInput(otherSlug), otherAdminEmail)).companyID;
}, networkHookTimeout);

afterAll(async () => {
	for (const heldCompanyID of [companyID, otherCompanyID].filter(Boolean)) {
		const { data: members } = await client.from('member').select('user_id').eq('company_id', heldCompanyID);
		await client.from('company').delete().eq('id', heldCompanyID);
		for (const member of members ?? []) {
			if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
		}
	}
	for (const accountID of [...fillerAccountIDs, ...strayAccountIDs]) await client.auth.admin.deleteUser(accountID);
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

	test('resetting a password issues a new one and leaves the member where it was', async () => {
		const memberID = await addMember(client, companyID, colleagueEmail);
		const invitation = await inviteMember(client, memberID);
		await client.from('member').update({ status: 'active' }).eq('id', memberID);

		const reset = await resetMemberPassword(client, memberID);
		const { data } = await client.from('member').select('status').eq('id', memberID).single();

		expect(reset.email).toBe(colleagueEmail);
		expect(reset.temporaryPassword).not.toBe(invitation.temporaryPassword);
		expect(data!.status).toBe('active');
	});

	test('a platform identity resolves back to its member', async () => {
		const memberID = (await memberOf(colleagueEmail)).id;
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

describe('an address belongs to one company', () => {
	test('another company cannot add a member this company holds, and the member stays', async () => {
		await expect(addMember(client, otherCompanyID, adminEmail, { isAdmin: false })).rejects.toBeInstanceOf(
			AddressBelongsToAnotherCompany,
		);

		const admin = await memberOf(adminEmail);
		expect(admin.company_id).toBe(companyID);
		expect(admin.is_admin).toBe(true);
	});

	test('inviting someone already active here is refused and keeps their role', async () => {
		await client.from('member').update({ status: 'active' }).eq('email', adminEmail);

		await expect(addMember(client, companyID, adminEmail)).rejects.toBeInstanceOf(AlreadyAMember);

		const admin = await memberOf(adminEmail);
		expect(admin.is_admin).toBe(true);
		expect(admin.status).toBe('active');
	});

	test('founding with an invitee another company holds founds nothing and moves nobody', async () => {
		const refusedSlug = `${slug}-refused`;
		await expect(
			foundCompany(
				client,
				{ accountID: crypto.randomUUID(), email: `${refusedSlug}-founder@example.test` },
				companyInput(refusedSlug),
				[colleagueEmail],
			),
		).rejects.toBeInstanceOf(AddressBelongsToAnotherCompany);

		const { data: refused } = await client.from('company').select('id').eq('slug', refusedSlug).maybeSingle();
		expect(refused).toBeNull();
		expect((await memberOf(colleagueEmail)).company_id).toBe(companyID);
	});

	test(
		'an invitation finds its account after more than one page of accounts',
		async () => {
			const late = await client.auth.admin.createUser({ email: lateEmail, email_confirm: true });
			if (late.error) throw new Error(late.error.message);
			for (let filler = 0; filler < fillerCount; filler += 1) {
				const created = await client.auth.admin.createUser({
					email: `${slug}-filler-${filler}@example.test`,
					email_confirm: true,
				});
				if (created.error) throw new Error(created.error.message);
				fillerAccountIDs.push(created.data.user.id);
			}

			const memberID = await addMember(client, companyID, lateEmail);
			const invitation = await inviteMember(client, memberID);

			expect(invitation.email).toBe(lateEmail);
			expect(invitation.temporaryPassword).not.toBe('');
		},
		networkHookTimeout,
	);
});

describe('a password is issued only to the account a member holds', () => {
	test("a company computer's address is never taken in as a member", async () => {
		await expect(addMember(client, companyID, hostAddressOf(otherCompanyID))).rejects.toBeInstanceOf(
			AddressBelongsToAnotherCompany,
		);

		const { data } = await client.from('member').select('id').eq('email', hostAddressOf(otherCompanyID));
		expect(data).toEqual([]);
	});

	test("a company computer's account is never handed a password, even through a member row naming it", async () => {
		const hostAddress = hostAddressOf(otherCompanyID);
		const host = await client.auth.admin.createUser({
			email: hostAddress,
			email_confirm: true,
			app_metadata: { company_id: otherCompanyID },
		});
		if (host.error) throw new Error(host.error.message);
		strayAccountIDs.push(host.data.user.id);
		const { data: planted, error } = await client
			.from('member')
			.insert({ company_id: companyID, email: hostAddress, status: 'pending' })
			.select('id')
			.single();
		if (error) throw new Error(error.message);

		await expect(resetMemberPassword(client, planted.id)).rejects.toBeInstanceOf(AddressBelongsToAnotherCompany);
		await expect(inviteMember(client, planted.id)).rejects.toBeInstanceOf(AddressBelongsToAnotherCompany);

		await client.from('member').delete().eq('id', planted.id);
	});

	test('an account another member holds is not handed to whoever invites its address', async () => {
		const heldEmail = `${otherSlug}-held@example.test`;
		const movedEmail = `${otherSlug}-moved@example.test`;
		const holderID = await addMember(client, otherCompanyID, heldEmail);
		await inviteMember(client, holderID);
		await client.from('member').update({ email: movedEmail }).eq('id', holderID);

		const strangerID = await addMember(client, companyID, heldEmail);

		await expect(inviteMember(client, strangerID)).rejects.toBeInstanceOf(AddressBelongsToAnotherCompany);
		await client.from('member').delete().eq('id', strangerID);
	});
});

describe('an administrator is one who administers now', () => {
	async function administratorClient(memberID: string) {
		const { data, error } = await client.from('member').select('user_id, email').eq('id', memberID).single();
		if (error) throw new Error(error.message);
		const token = await recordTokenFor(signingKey, projectURL, { userID: data.user_id, email: data.email });
		return asMember({ projectURL, publishableKey }, token.accessToken);
	}

	test('an active administrator is answered, and one who has left is not', async () => {
		const leavingEmail = `${slug}-leaving-admin@example.test`;
		const memberID = await addMember(client, companyID, leavingEmail, { isAdmin: true });
		await inviteMember(client, memberID);
		await client.from('member').update({ status: 'active' }).eq('id', memberID);
		const administrator = await administratorClient(memberID);

		expect(await adminCallerOf(administrator)).toEqual({ memberID, companyID });

		await client.from('member').update({ status: 'withdrawn' }).eq('id', memberID);
		expect(await adminCallerOf(administrator)).toBeNull();

		await client.from('member').update({ status: 'departed' }).eq('id', memberID);
		expect(await adminCallerOf(administrator)).toBeNull();
	});

	test('an administrator who has not arrived yet does not administer', async () => {
		const invitedEmail = `${slug}-invited-admin@example.test`;
		const memberID = await addMember(client, companyID, invitedEmail, { isAdmin: true });
		await inviteMember(client, memberID);

		expect(await adminCallerOf(await administratorClient(memberID))).toBeNull();
	});
});

describe('somebody who leaves stops signing in', () => {
	function aBrowser() {
		return createClient(projectURL, publishableKey, { auth: { persistSession: false, autoRefreshToken: false } });
	}

	async function signedInAs(email: string, password: string) {
		const browser = aBrowser();
		const signedIn = await browser.auth.signInWithPassword({ email, password });
		if (signedIn.error) throw new Error(signedIn.error.message);
		return { browser, refreshToken: signedIn.data.session.refresh_token };
	}

	async function canSignIn(email: string, password: string): Promise<boolean> {
		const { error } = await aBrowser().auth.signInWithPassword({ email, password });
		return error === null;
	}

	async function canRefresh(refreshToken: string): Promise<boolean> {
		const { error } = await aBrowser().auth.refreshSession({ refresh_token: refreshToken });
		return error === null;
	}

	test('removing a member refuses their password and their refresh token, and inviting them again lifts it', async () => {
		const email = `${slug}-removed@example.test`;
		const memberID = await addMember(client, companyID, email);
		const { temporaryPassword } = await inviteMember(client, memberID);
		const { refreshToken } = await signedInAs(email, temporaryPassword);

		await removeMember(client, companyID, memberID);

		expect(await canSignIn(email, temporaryPassword)).toBe(false);
		expect(await canRefresh(refreshToken)).toBe(false);

		const reinvited = await inviteMember(client, memberID);
		expect(await canSignIn(email, reinvited.temporaryPassword)).toBe(true);
	});

	test('a departure written to the record closes the account, and a return opens it', async () => {
		const email = `${slug}-departed@example.test`;
		const memberID = await addMember(client, companyID, email);
		const { temporaryPassword } = await inviteMember(client, memberID);

		await client.from('member').update({ status: 'departed' }).eq('id', memberID);
		await settleSignInOfMember(client, memberID);
		expect(await canSignIn(email, temporaryPassword)).toBe(false);

		await client.from('member').update({ status: 'active' }).eq('id', memberID);
		await settleSignInOfMember(client, memberID);
		expect(await canSignIn(email, temporaryPassword)).toBe(true);
	});

	test('somebody who departed before departures closed sign-in is closed by the backfill, once', async () => {
		const email = `${slug}-departed-before@example.test`;
		const memberID = await addMember(client, companyID, email);
		const { temporaryPassword } = await inviteMember(client, memberID);
		const { refreshToken } = await signedInAs(email, temporaryPassword);
		await client.from('member').update({ status: 'departed' }).eq('id', memberID);
		expect(await canSignIn(email, temporaryPassword)).toBe(true);

		const rehearsal = await closeSignInOfMembersWhoLeft(client, { isDryRun: true });
		expect(rehearsal.closed.map((account) => account.memberID)).toContain(memberID);
		expect(await canSignIn(email, temporaryPassword)).toBe(true);

		const backfill = await closeSignInOfMembersWhoLeft(client, { isDryRun: false });
		expect(backfill.closed.map((account) => account.memberID)).toContain(memberID);
		expect(await canSignIn(email, temporaryPassword)).toBe(false);
		expect(await canRefresh(refreshToken)).toBe(false);

		const again = await closeSignInOfMembersWhoLeft(client, { isDryRun: false });
		expect(again.closed.map((account) => account.memberID)).not.toContain(memberID);
		expect(again.alreadyClosed.map((account) => account.memberID)).toContain(memberID);
	});
});

describe('a member found by their address', () => {
	test('is found by that address alone, never by a pattern spelled in it', async () => {
		const underscored = await addMember(client, companyID, `${slug}-a_b@example.test`);
		await addMember(client, companyID, `${slug}-axb@example.test`);

		expect(await memberOfCompanyByEmail(client, companyID, `${slug}-A_B@example.test`)).toBe(underscored);
		expect(await memberOfCompanyByEmail(client, companyID, `${slug}-a%@example.test`)).toBeNull();
		expect(await memberOfCompanyByEmail(client, companyID, `${slug}-a_c@example.test`)).toBeNull();
	});
});
