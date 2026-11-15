import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';
import { heldToTheContract } from './tool-answers';
import { leavesTaskLabelsUndecided } from '../../src/lib/server/public-api/record/task-labels';
import { defaultCreditUsageLimit } from '../../src/lib/credit-policy';
import { companyBoxesOf, companyModelKeyFacts } from '../../src/lib/server/box';
import { keptModelKeyReport } from '../../../supabase/functions/_shared/model-key-report';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { runToolOverTheRecord } = await import('../../src/lib/server/public-api/record');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `company-credits-${Date.now()}`;
const now = new Date();

let companyID = '';
let sampleID = '';
let adminID = '';
let sample: ReturnType<typeof asMember>;
let admin: ReturnType<typeof asMember>;

async function signedInMember(memberID: string, email: string): Promise<ReturnType<typeof asMember>> {
	const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Company Credits Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;
	sampleID = await addMember(client, companyID, `${slug}-sample@example.test`);
	await client.from('member').update({ name: '이샘플' }).eq('id', sampleID);
	await client.from('member').update({ name: '최견본' }).eq('id', adminID);
	sample = await signedInMember(sampleID, `${slug}-sample@example.test`);
	admin = await signedInMember(adminID, `${slug}-admin@example.test`);

	const { error } = await client.rpc('company_subscription_invoice_paid', {
		target_company: companyID,
		target_event: `evt_${slug}`,
		target_customer: `cus_${slug}`,
		target_subscription: `sub_${slug}`,
		granted_credits: 20000,
		opens_period: true,
		period_ends_at: new Date(now.getTime() + 30 * 86_400_000).toISOString()
	});
	if (error) throw new Error(error.message);
	const { error: statusError } = await client.rpc('company_subscription_changed', {
		target_company: companyID,
		target_customer: `cus_${slug}`,
		target_subscription: `sub_${slug}`,
		target_status: 'active',
		status_at: now.toISOString()
	});
	if (statusError) throw new Error(statusError.message);
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

async function asSample(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(sample, client, sampleID, name, input, now, leavesTaskLabelsUndecided));
}

async function asAdmin(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(admin, client, adminID, name, input, now, leavesTaskLabelsUndecided));
}

function resultOf(answer: { body: unknown }): Record<string, unknown> {
	return (answer.body as { result: Record<string, unknown> }).result;
}

describe('company_credits_get', () => {
	test('an administrator reads the subscription, the seats and every member', async () => {
		const credits = resultOf(await asAdmin('company_credits_get'));
		expect(credits.paidBy).toBe('subscription');
		expect(credits.subscriptionCredits).toBe(20000);
		expect(credits.billedSeats).toBe(2);
		expect(credits.usageLimit).toBe(defaultCreditUsageLimit);
		expect((credits.members as { name: string }[]).map((member) => member.name).sort()).toEqual(['이샘플', '최견본']);
	});

	test('a member reads only their own usage', async () => {
		const credits = resultOf(await asSample('company_credits_get'));
		expect(credits.paidBy).toBe('subscription');
		expect(credits.myCreditsUsed).toBe(0);
		expect(credits.billedSeats).toBeNull();
		expect(credits.members).toEqual([]);
	});
});

describe('the per-member credit limit', () => {
	test('an administrator sets one limit every member answers to', async () => {
		const updated = await asAdmin('company_settings_update', { creditUsageLimit: 25000 });
		expect(updated.status).toBe(200);
		expect(resultOf(await asSample('company_credits_get')).usageLimit).toBe(25000);
	});

	test('a member who does not administer cannot change it', async () => {
		const refused = await asSample('company_settings_update', { creditUsageLimit: 1 });
		expect(refused.status).toBeGreaterThanOrEqual(400);
		expect(resultOf(await asAdmin('company_credits_get')).usageLimit).toBe(25000);
	});
});

describe('who pays for the models', () => {
	const sealingKey = 'A'.repeat(43);
	const sealedModelKey = { version: 1, recipient: sealingKey, enc: sealingKey, ciphertext: 'c2VhbGVk' };

	afterAll(async () => {
		await client.from('box_keyring').delete().eq('company_id', companyID);
		await client.from('company').update({ uses_own_model_key: true, host_holds_model_key: null }).eq('id', companyID);
	}, networkHookTimeout);

	test('the company key pays while it is held and in use, and credits pay once it is turned off', async () => {
		const { error } = await client.from('box_keyring').insert({ company_id: companyID, sealing_key: sealingKey, sealed_model_key: sealedModelKey });
		if (error) throw new Error(error.message);
		expect((await asAdmin('company_settings_update', { usesOwnModelKey: true })).status).toBe(200);
		expect(resultOf(await asSample('company_credits_get')).paidBy).toBe('company_key');

		expect((await asAdmin('company_settings_update', { usesOwnModelKey: false })).status).toBe(200);
		expect((await companyModelKeyFacts(client, companyID)).usesOwnModelKey).toBe(false);
		expect(resultOf(await asSample('company_credits_get')).paidBy).toBe('subscription');
		expect((await client.from('box_keyring').select('sealed_model_key').eq('company_id', companyID).single()).data?.sealed_model_key).toEqual(sealedModelKey);
	});

	test('a key typed into the computer pays once the computer says it holds one, and is unknown until then', async () => {
		await client.from('box_keyring').delete().eq('company_id', companyID);
		await client.from('company').update({ uses_own_model_key: true, host_holds_model_key: null }).eq('id', companyID);
		expect((await companyBoxesOf(client, companyID)).paidBy).toBe('unknown');
		expect(resultOf(await asSample('company_credits_get')).paidBy).toBe('subscription');

		const answered = await keptModelKeyReport(client, companyID, { holdsModelKey: true });
		expect(await answered.json()).toEqual({ usesOwnModelKey: true });
		expect(await companyBoxesOf(client, companyID)).toMatchObject({ hasModelKey: false, hostHoldsModelKey: true, paidBy: 'company_key' });
		expect(resultOf(await asSample('company_credits_get')).paidBy).toBe('company_key');

		await keptModelKeyReport(client, companyID, { holdsModelKey: false });
		expect((await companyBoxesOf(client, companyID)).paidBy).toBe('subscription');
		const refused = await keptModelKeyReport(client, companyID, {}).catch((refusal: unknown) => refusal);
		expect(refused instanceof Response ? refused.status : refused).toBe(400);
		await client.from('company').update({ uses_own_model_key: false }).eq('id', companyID);
	});

	test('a member who does not administer cannot choose who pays', async () => {
		const refused = await asSample('company_settings_update', { usesOwnModelKey: true });
		expect(refused.status).toBeGreaterThanOrEqual(400);
		expect((await companyModelKeyFacts(client, companyID)).usesOwnModelKey).toBe(false);
	});
});
