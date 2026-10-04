import { describe, expect, test } from 'bun:test';
import {
	channelMessageCacheGeneration,
	clearChannelMessageCache,
	getCachedMessages,
	getCachedReaderID,
	setCachedMessages,
	setCachedReaderID
} from '../../../src/lib/components/channel/channel-message-cache';

describe('the cache a reopened channel draws from', () => {
	test('remembers who is reading, so a redraw knows which side is theirs', () => {
		setCachedReaderID('member:m1');
		expect(getCachedReaderID()).toBe('member:m1');
	});

	test('hands back the messages it was given for that channel', () => {
		const messages = [
			{ id: 'p1', sender: { id: 'member:m1', name: '이샘플' }, text: '안녕', sentAt: '2026-08-05T10:00:00Z' }
		];
		setCachedMessages('c1', messages);
		expect(getCachedMessages('c1')).toEqual(messages);
	});

	test('forgets both messages and reader when the account signs out', () => {
		const previousGeneration = channelMessageCacheGeneration();
		setCachedReaderID('member:old');
		setCachedMessages('old-channel', []);
		clearChannelMessageCache();
		expect(channelMessageCacheGeneration()).toBe(previousGeneration + 1);
		expect(getCachedReaderID()).toBe('');
		expect(getCachedMessages('old-channel')).toBeUndefined();
	});

	test('a channel nobody has opened has nothing cached', () => {
		expect(getCachedMessages('never-opened')).toBe(undefined);
		expect(getCachedMessages(undefined)).toBe(undefined);
	});
});
