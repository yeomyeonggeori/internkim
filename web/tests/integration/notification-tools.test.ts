import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';
import { heldToTheContract } from './tool-answers';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { runToolOverTheRecord } = await import('../../src/lib/server/public-api/record');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `notification-tools-${Date.now()}`;
const now = new Date();
const subscription = 'https://push.example.test/subscription-1';

let companyID = '';
let sampleID = '';
let adminID = '';
let sample: ReturnType<typeof asMember>;
let admin: ReturnType<typeof asMember>;

async function signedInMember(memberID: string, email: string): Promise<ReturnType<typeof asMember>> {
	const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Notification Tools Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
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
	return heldToTheContract(name, await runToolOverTheRecord(sample, client, sampleID, name, input, now));
}

async function asAdmin(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(admin, client, adminID, name, input, now));
}

type NotificationChoice = { category: string; isOn: boolean; isChoosable: boolean };

type SettingsResult = { categories: NotificationChoice[]; mutedConversationIDs: string[] };

type MutingResult = { conversationID: string; isMuted: boolean; mutedConversationIDs: string[] };

type ReachabilityResult = { serverKey: string; isServerKeyVaulted: boolean; hasClaimedDevice: boolean };

function resultOf<Result>(answer: { body: unknown }): Result {
	return (answer.body as { result: Result }).result;
}

function errorOf(answer: { body: unknown }): string {
	return (answer.body as { error?: string }).error ?? '';
}

function choiceOf(settings: SettingsResult, category: string): NotificationChoice {
	const choice = settings.categories.find((held) => held.category === category);
	if (!choice) throw new Error(`the settings say nothing about ${category}`);
	return choice;
}

describe('what a person is told about', () => {
	test('is answered with the default nobody has chosen against', async () => {
		const answered = await asSample('notification_settings_get');
		const settings = resultOf<SettingsResult>(answered);

		expect(answered.status).toBe(200);
		expect(choiceOf(settings, 'message').isOn).toBe(true);
		expect(choiceOf(settings, 'mail').isOn).toBe(false);
		expect(settings.mutedConversationIDs).toEqual([]);
	});

	test('says which categories the requester may choose', async () => {
		const mine = resultOf<SettingsResult>(await asSample('notification_settings_get'));
		const administrators = resultOf<SettingsResult>(await asAdmin('notification_settings_get'));

		expect(choiceOf(mine, 'leave').isChoosable).toBe(false);
		expect(choiceOf(mine, 'message').isChoosable).toBe(true);
		expect(choiceOf(administrators, 'leave').isChoosable).toBe(true);
	});

	test('changes only the categories the call names', async () => {
		const changed = resultOf<SettingsResult>(
			await asSample('notification_settings_set', { turnOn: ['mail'], turnOff: ['task'] })
		);

		expect(choiceOf(changed, 'mail').isOn).toBe(true);
		expect(choiceOf(changed, 'task').isOn).toBe(false);
		expect(choiceOf(changed, 'message').isOn).toBe(true);

		const readBack = resultOf<SettingsResult>(await asSample('notification_settings_get'));
		expect(choiceOf(readBack, 'mail').isOn).toBe(true);
		expect(choiceOf(readBack, 'task').isOn).toBe(false);
	});

	test('refuses a category the requester may not choose, and names it', async () => {
		const refused = await asSample('notification_settings_set', { turnOff: ['leave'] });

		expect(refused.status).toBe(403);
		expect(errorOf(refused)).toContain('leave');

		const kept = resultOf<SettingsResult>(await asAdmin('notification_settings_set', { turnOff: ['leave'] }));
		expect(choiceOf(kept, 'leave').isOn).toBe(false);
	});

	test('refuses a change that names no category', async () => {
		const refused = await asSample('notification_settings_set', {});

		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toContain('at least one category');
	});
});

describe('a conversation the requester stopped being told about', () => {
	test('is muted, answered back, and unmuted again', async () => {
		const muted = resultOf<MutingResult>(await asSample('conversation_mute', { conversationID: 'conversation-1' }));
		expect(muted.isMuted).toBe(true);
		expect(muted.mutedConversationIDs).toEqual(['conversation-1']);

		const settings = resultOf<SettingsResult>(await asSample('notification_settings_get'));
		expect(settings.mutedConversationIDs).toEqual(['conversation-1']);

		const unmuted = resultOf<MutingResult>(await asSample('conversation_unmute', { conversationID: 'conversation-1' }));
		expect(unmuted.isMuted).toBe(false);
		expect(unmuted.mutedConversationIDs).toEqual([]);
	});

	test('belongs to one person, so a colleague reads none of it', async () => {
		await asSample('conversation_mute', { conversationID: 'conversation-2' });

		const colleague = resultOf<SettingsResult>(await asAdmin('notification_settings_get'));
		expect(colleague.mutedConversationIDs).toEqual([]);

		await asSample('conversation_unmute', { conversationID: 'conversation-2' });
	});
});

describe('the browser subscription push is delivered to', () => {
	test('is claimed, read back, and released', async () => {
		const before = resultOf<ReachabilityResult>(await asSample('push_reachability_get'));
		expect(before.hasClaimedDevice).toBe(false);

		const claimed = resultOf<ReachabilityResult>(
			await asSample('push_device_claim', {
				endpoint: subscription,
				publicKey: 'a-public-key',
				authenticationSecret: 'an-authentication-secret'
			})
		);
		expect(claimed.hasClaimedDevice).toBe(true);

		const released = resultOf<ReachabilityResult>(
			await asSample('push_device_release', { endpoint: subscription })
		);
		expect(released.hasClaimedDevice).toBe(false);
	});

	test('is refused when the call carries no keys to encrypt to', async () => {
		const refused = await asSample('push_device_claim', { endpoint: subscription });

		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toContain('keys');
	});

	test('answers no server key while this company has none in the vault', async () => {
		const answered = resultOf<ReachabilityResult>(await asSample('push_reachability_get'));

		expect(answered.serverKey).toBe('');
		expect(answered.isServerKeyVaulted).toBe(false);
	});
});
