import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { z } from 'zod';
import { channelOrderResultSchema, channelPinResultSchema } from '../../src/lib/server/public-api/catalog/channel-order';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';
import { heldToTheContract } from './tool-answers';
import { leavesTaskLabelsUndecided } from '../../src/lib/server/public-api/record/task-labels';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { runToolOverTheRecord } = await import('../../src/lib/server/public-api/record');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `channel-order-tools-${Date.now()}`;
const now = new Date();

let companyID = '';
let sampleID = '';
let adminID = '';
let sample: ReturnType<typeof asMember>;
let admin: ReturnType<typeof asMember>;

async function signedInMember(memberID: string, email: string): Promise<ReturnType<typeof asMember>> {
	const { data: account, error } = await client.auth.admin.createUser({ email, email_confirm: true });
	if (error || !account.user) throw new Error(`could not create the account for ${email}: ${error?.message ?? 'no user came back'}`);
	await client.from('member').update({ user_id: account.user.id, status: 'active' }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Channel Order Tools Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;

	sampleID = await addMember(client, companyID, `${slug}-sample@example.test`);
	await client.from('member').update({ name: '이샘플' }).eq('id', sampleID);
	await client.from('member').update({ name: '최견본' }).eq('id', adminID);

	sample = await signedInMember(sampleID, `${slug}-sample@example.test`);
	admin = await signedInMember(adminID, `${slug}-admin@example.test`);
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

function orderOf(answer: { body: unknown }) {
	return z.object({ result: channelOrderResultSchema }).parse(answer.body).result;
}

function pinOf(answer: { body: unknown }) {
	return z.object({ result: channelPinResultSchema }).parse(answer.body).result;
}

function errorOf(answer: { body: unknown }): string {
	return z.object({ error: z.string() }).parse(answer.body).error;
}

describe('the channels a requester pins and orders', () => {
	test('are pinned in order, reordered, and unpinned back to their place', async () => {
		expect(orderOf(await asSample('channel_order_get'))).toEqual({ pinnedChannels: [], channelOrder: [] });

		await asSample('channel_pin', { channelID: 'channel-1' });
		const pinned = pinOf(await asSample('channel_pin', { channelID: 'channel-2' }));
		expect(pinned.isPinned).toBe(true);
		expect(pinned.pinnedChannels).toEqual(['channel-1', 'channel-2']);

		const ordered = orderOf(await asSample('channel_order_set', { channelOrder: ['channel-3', 'channel-1', 'channel-2'] }));
		expect(ordered.channelOrder).toEqual(['channel-3', 'channel-1', 'channel-2']);

		const reordered = orderOf(await asSample('channel_order_set', { pinnedChannels: ['channel-2', 'channel-1'] }));
		expect(reordered.pinnedChannels).toEqual(['channel-2', 'channel-1']);

		const unpinned = pinOf(await asSample('channel_unpin', { channelID: 'channel-2' }));
		expect(unpinned.isPinned).toBe(false);
		expect(unpinned.pinnedChannels).toEqual(['channel-1']);
		expect(unpinned.channelOrder).toEqual(['channel-3', 'channel-1', 'channel-2']);
	});

	test('belong to one person, so a colleague reads none of them', async () => {
		await asSample('channel_pin', { channelID: 'channel-4' });

		expect(orderOf(await asAdmin('channel_order_get'))).toEqual({ pinnedChannels: [], channelOrder: [] });
	});

	test('refuse an order change that names nothing to order', async () => {
		const refused = await asSample('channel_order_set', {});

		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toContain('names the pinned channels, the channel order, or both');
	});
});
