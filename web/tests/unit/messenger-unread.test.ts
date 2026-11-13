import { describe, expect, test } from 'bun:test';
import { createReadMarker, latestSettledOf } from '../../src/lib/messenger/conversation-read-marker';
import { conversationNameClass, isUnreadEmphasized, unreadBadgeLabel } from '../../src/lib/messenger/unread-badge';

const exactly = (value: number) => ({ status: 'exact' as const, value });

describe('unread badge', () => {
	test('shows nothing for a conversation with nothing unread, or one whose count is unknown', () => {
		expect(unreadBadgeLabel(undefined)).toBe('');
		expect(unreadBadgeLabel(exactly(0))).toBe('');
		expect(unreadBadgeLabel({ status: 'unknown' })).toBe('');
	});

	test('shows an exact count, a lower bound with a plus, and stops counting past 99', () => {
		expect(unreadBadgeLabel(exactly(3))).toBe('3');
		expect(unreadBadgeLabel(exactly(99))).toBe('99');
		expect(unreadBadgeLabel(exactly(100))).toBe('99+');
		expect(unreadBadgeLabel({ status: 'at_least', value: 7 })).toBe('7+');
	});

	test('a muted conversation still counts but is not emphasized', () => {
		expect(isUnreadEmphasized(exactly(4), false)).toBe(true);
		expect(isUnreadEmphasized(exactly(4), true)).toBe(false);
		expect(isUnreadEmphasized({ status: 'unknown' }, false)).toBe(false);
	});

	test('a read conversation is dimmed unless it is the open one', () => {
		expect(conversationNameClass(exactly(4), false, false)).toBe('font-bold');
		expect(conversationNameClass(exactly(0), false, false)).toBe('text-sidebar-foreground/70');
		expect(conversationNameClass(exactly(4), true, false)).toBe('text-sidebar-foreground/70');
		expect(conversationNameClass(exactly(0), false, true)).toBe('');
	});
});

describe('latest message read', () => {
	test('is the newest message that has settled', () => {
		expect(
			latestSettledOf([
				{ id: 'one', sentAt: '2026-09-28T01:00:00Z' },
				{ id: 'three', sentAt: '2026-09-28T03:00:00Z' },
				{ id: 'pending-four', sentAt: '2026-09-28T04:00:00Z' },
				{ id: 'two', sentAt: '2026-09-28T02:00:00Z' }
			])?.id
		).toBe('three');
		expect(latestSettledOf([])).toBeUndefined();
	});
});

describe('read marker', () => {
	test('records each conversation and each thread once per message seen', async () => {
		const recorded: string[] = [];
		const markRead = createReadMarker(async (conversationID, messageID, threadRootID) => {
			recorded.push(`${conversationID}/${threadRootID ?? ''}@${messageID}`);
		});
		expect(await markRead('channel-1', 'message-1')).toBe(true);
		expect(await markRead('channel-1', 'message-1')).toBe(false);
		expect(await markRead('channel-1', 'reply-1', 'message-1')).toBe(true);
		expect(await markRead('channel-1', 'message-2')).toBe(true);
		expect(recorded).toEqual(['channel-1/@message-1', 'channel-1/message-1@reply-1', 'channel-1/@message-2']);
	});

	test('a failed record is tried again the next time', async () => {
		let shouldFail = true;
		const markRead = createReadMarker(async () => {
			if (shouldFail) throw new Error('the messenger did not answer');
		});
		await expect(markRead('channel-1', 'message-1')).rejects.toThrow('did not answer');
		shouldFail = false;
		expect(await markRead('channel-1', 'message-1')).toBe(true);
	});
});
