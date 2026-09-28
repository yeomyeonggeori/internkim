import { describe, expect, test } from 'bun:test';
import { createConversationReadMarker, latestSentAtOf } from '../../src/lib/messenger/conversation-read-marker';
import { isUnreadEmphasized, unreadBadgeLabel } from '../../src/lib/messenger/unread-badge';

describe('unread badge', () => {
	test('shows nothing for a conversation with nothing unread', () => {
		expect(unreadBadgeLabel(undefined)).toBe('');
		expect(unreadBadgeLabel(0)).toBe('');
	});

	test('shows the count, and stops counting past 99', () => {
		expect(unreadBadgeLabel(3)).toBe('3');
		expect(unreadBadgeLabel(99)).toBe('99');
		expect(unreadBadgeLabel(100)).toBe('99+');
	});

	test('a muted conversation still counts but is not emphasized', () => {
		expect(isUnreadEmphasized(4, false)).toBe(true);
		expect(isUnreadEmphasized(4, true)).toBe(false);
		expect(isUnreadEmphasized(0, false)).toBe(false);
	});
});

describe('latest message read', () => {
	test('is the newest settled message in the timeline', () => {
		expect(
			latestSentAtOf([
				{ id: 'one', sentAt: '2026-09-28T01:00:00Z' },
				{ id: 'three', sentAt: '2026-09-28T03:00:00Z' },
				{ id: 'two', sentAt: '2026-09-28T02:00:00Z' }
			])
		).toBe('2026-09-28T03:00:00Z');
	});

	test('a message still sending or a thread reply does not move it', () => {
		expect(
			latestSentAtOf([
				{ id: 'one', sentAt: '2026-09-28T01:00:00Z' },
				{ id: 'pending-two', sentAt: '2026-09-28T02:00:00Z' },
				{ id: 'reply', sentAt: '2026-09-28T03:00:00Z', threadRootId: 'one' }
			])
		).toBe('2026-09-28T01:00:00Z');
		expect(latestSentAtOf([])).toBe('');
	});
});

describe('conversation read marker', () => {
	test('records each conversation once per newer message', async () => {
		const recorded: string[] = [];
		const markReadThrough = createConversationReadMarker(async (conversationID, readAt) => {
			recorded.push(`${conversationID}@${readAt}`);
		});
		expect(await markReadThrough('channel-1', '2026-09-28T01:00:00Z')).toBe(true);
		expect(await markReadThrough('channel-1', '2026-09-28T01:00:00Z')).toBe(false);
		expect(await markReadThrough('channel-1', '2026-09-28T00:30:00Z')).toBe(false);
		expect(await markReadThrough('channel-2', '2026-09-28T00:30:00Z')).toBe(true);
		expect(await markReadThrough('channel-1', '2026-09-28T02:00:00Z')).toBe(true);
		expect(recorded).toEqual([
			'channel-1@2026-09-28T01:00:00Z',
			'channel-2@2026-09-28T00:30:00Z',
			'channel-1@2026-09-28T02:00:00Z'
		]);
	});

	test('a failed record is tried again the next time', async () => {
		let shouldFail = true;
		const markReadThrough = createConversationReadMarker(async () => {
			if (shouldFail) throw new Error('the messenger did not answer');
		});
		await expect(markReadThrough('channel-1', '2026-09-28T01:00:00Z')).rejects.toThrow('did not answer');
		shouldFail = false;
		expect(await markReadThrough('channel-1', '2026-09-28T01:00:00Z')).toBe(true);
	});
});
