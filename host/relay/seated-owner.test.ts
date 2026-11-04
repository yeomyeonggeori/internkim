import { afterAll, afterEach, describe, expect, spyOn, test } from 'bun:test';
import { seatedOwnerMuter, seatedOwnerOf, type SeatedOwnerMuting } from './seated-owner';

const seated = { conversationID: 'channel-1', externalID: 'npub-admin' };

describe('the owner a channel owner add seated', () => {
	test('is the one the messenger says was new to the channel', () => {
		expect(seatedOwnerOf(seated, { status: 200, body: { newToChannel: true } })).toEqual(seated);
	});

	test('is nobody when the owner was in the channel, the add was refused, or the messenger said nothing', () => {
		expect(seatedOwnerOf(seated, { status: 200, body: { newToChannel: false } })).toBeNull();
		expect(seatedOwnerOf(seated, { status: 403, body: { newToChannel: true } })).toBeNull();
		expect(seatedOwnerOf(seated, { status: 200, body: {} })).toBeNull();
	});

	test('is nobody when the call named no channel or no person', () => {
		expect(seatedOwnerOf({ externalID: 'npub-admin' }, { status: 200, body: { newToChannel: true } })).toBeNull();
		expect(seatedOwnerOf({ conversationID: 'channel-1' }, { status: 200, body: { newToChannel: true } })).toBeNull();
	});
});

describe('muting a seated owner', () => {
	const errors = spyOn(console, 'error').mockImplementation(() => {});

	afterEach(() => {
		errors.mockClear();
	});

	afterAll(() => {
		errors.mockRestore();
	});

	function mutingThatRecords(memberIDs: Record<string, string>, refusal?: Error) {
		const muted: { memberID: string; conversationID: string }[] = [];
		const muting: SeatedOwnerMuting = {
			memberIDOf: async (externalID) => memberIDs[externalID] ?? null,
			muteFor: async (memberID, conversationID) => {
				if (refusal) throw refusal;
				muted.push({ memberID, conversationID });
			}
		};
		return { muted, mute: seatedOwnerMuter(muting) };
	}

	test('mutes the channel for the member who holds that messenger account', async () => {
		const { muted, mute } = mutingThatRecords({ 'npub-admin': 'member-admin' });

		await mute(seated);

		expect(muted).toEqual([{ memberID: 'member-admin', conversationID: 'channel-1' }]);
		expect(errors).not.toHaveBeenCalled();
	});

	test('says so when nobody in the company holds that messenger account', async () => {
		const { muted, mute } = mutingThatRecords({});

		await mute(seated);

		expect(muted).toEqual([]);
		expect(errors).toHaveBeenCalledWith('a seated owner was not muted', {
			...seated,
			reason: 'nobody in this company holds that messenger account'
		});
	});

	test('says so when the central plane refuses the mute, and leaves the add standing', async () => {
		const { mute } = mutingThatRecords({ 'npub-admin': 'member-admin' }, new Error('the central plane answered 502 for /api/agent/conversation-mute'));

		await mute(seated);

		expect(errors).toHaveBeenCalledWith('a seated owner was not muted', {
			...seated,
			reason: 'the central plane answered 502 for /api/agent/conversation-mute'
		});
	});
});
