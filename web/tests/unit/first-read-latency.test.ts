import { describe, expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../support/in-its-own-process';

async function outcome(scenario: string) {
	const output = await runInItsOwnProcess(new URL('./first-read-latency.fixture.ts', import.meta.url), 'browser', scenario);
	return JSON.parse(output);
}

describe('first-read request dependencies', () => {
	test('mail reads its account and each fresh list once, including an explicit refresh', async () => {
		expect(await outcome('mail')).toEqual({
			first: { accounts: 1, mailboxes: 1, messages: 1, visible: 'Fresh message' },
			refresh: { accounts: 2, mailboxes: 2, messages: 2, visible: 'Fresh message' }
		});
	}, ownProcessTimeoutMilliseconds);
	test('messenger starts independent directory queries together', async () => {
		expect(await outcome('directory')).toEqual({ started: ['member', 'contact'], members: 0 });
	}, ownProcessTimeoutMilliseconds);
	test('closing the transport settles pending calls without waiting for their timeout', async () => {
		expect(await outcome('disconnect')).toEqual({ sent: 2, failures: ['HostUnreachableError', 'HostUnreachableError'] });
	}, ownProcessTimeoutMilliseconds);
	test('conversation posts load alongside the directory and render before emoji', async () => {
		expect(await outcome('conversation')).toEqual({ postsStarted: true, text: 'Message text', emojiBefore: 0, emojiAfter: 1 });
	}, ownProcessTimeoutMilliseconds);
	test('concurrent emoji callers share the pending name request', async () => {
		expect(await outcome('emoji')).toEqual({ requests: 1, secondResolvedEarly: false, image: 'data:image/png;base64,test' });
	}, ownProcessTimeoutMilliseconds);
	test('available file rows remain usable while the rest of the list loads', async () => {
		expect(await outcome('file-list')).toEqual({ visible: 'Available folder', busy: 'true' });
	}, ownProcessTimeoutMilliseconds);
	test('the company wire changes owner and token without accepting old results', async () => {
		expect(await outcome('ownership')).toEqual({
			oldFailure: 'HostUnreachableError', oldClosed: true, acceptedOldResult: false,
			secondStatus: 201, rotatedClosed: true, thirdStatus: 202, signOutClosed: true,
			signOutRefused: true, raceRefused: true, expectedOwnerRefused: true, connections: 3
		});
	}, ownProcessTimeoutMilliseconds);
	test('messenger caches follow verified ownership, reject late reads, and clear on logout', async () => {
		expect(await outcome('cache-scope')).toEqual({
			keysDiffer: true, containsToken: false, firstName: 'first', secondName: 'second',
			sameScopeReused: true, oldMessagesGone: true, oldStoredGone: true,
			lateRefused: true, correctAfterRace: true, reopenPreserved: true, refreshPreserved: true, logoutCleared: true
		});
	}, ownProcessTimeoutMilliseconds);
	test('a reopened messenger shows what it kept for the same account before verification, and nothing after logout', async () => {
		expect(await outcome('remembered-scope')).toEqual({
			otherAccountAdopted: null, adoptedKey: true, listBeforeVerification: ['kept-channel'], messagesBeforeVerification: ['안녕'],
			readerBeforeVerification: 'member:m1', storedLength: 50, logoutCleared: true
		});
	}, ownProcessTimeoutMilliseconds);
	test('old emoji, avatar and attachment work cannot repopulate a new account', async () => {
		expect(await outcome('asset-scope')).toEqual({ emoji: null, avatar: '', attachment: '', attachmentStatus: 'unasked', signedOldAssets: 0, newEmoji: 'new-image' });
	}, ownProcessTimeoutMilliseconds);
});
