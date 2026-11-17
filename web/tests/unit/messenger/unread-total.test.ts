import { describe, expect, test } from 'bun:test';
import type { ChannelSummary } from '../../../src/lib/components/channel/channel-api';
import { messengerUnreadTotal } from '../../../src/lib/messenger/unread-total';

const exactly = (value: number) => ({ status: 'exact' as const, value });
const direct = (id: string, unread: number, more: Partial<ChannelSummary> = {}): ChannelSummary => ({ id, name: id, kind: 'dm', unread: exactly(unread), ...more });
const channel = (id: string, unread: number): ChannelSummary => ({ id, name: id, kind: 'group', unread: exactly(unread) });
const mention = (conversationID: string, isUnread = true) => ({ conversationID, isUnread });
const nobodyMuted = new Set<string>();

describe('what the rail counts as unread in the messenger', () => {
	test('direct messages and channel mentions are counted, channel posts only leave a mark', () => {
		const total = messengerUnreadTotal([direct('dm-1', 2), direct('dm-2', 1), channel('channel-1', 7)], [mention('channel-1')], nobodyMuted);
		expect(total).toEqual({ addressedToMe: 4, hasChannelNews: true });
	});

	test('nothing unread leaves no count and no mark', () => {
		const total = messengerUnreadTotal([direct('dm-1', 0), channel('channel-1', 0)], [mention('channel-1', false)], nobodyMuted);
		expect(total).toEqual({ addressedToMe: 0, hasChannelNews: false });
	});

	test('a mention inside a direct message is not counted twice', () => {
		const total = messengerUnreadTotal([direct('dm-1', 1)], [mention('dm-1')], nobodyMuted);
		expect(total.addressedToMe).toBe(1);
	});

	test('a muted conversation adds neither a count nor a mark', () => {
		const muted = new Set(['dm-1', 'channel-1']);
		const total = messengerUnreadTotal([direct('dm-1', 5), channel('channel-1', 3)], [mention('channel-1')], muted);
		expect(total).toEqual({ addressedToMe: 0, hasChannelNews: false });
	});

	test('a hidden direct message and an unknown count add nothing', () => {
		const conversations = [direct('dm-hidden', 4, { isHidden: true }), direct('dm-unknown', 0, { unread: { status: 'unknown' } })];
		expect(messengerUnreadTotal(conversations, [], nobodyMuted).addressedToMe).toBe(0);
	});

	test('a lower bound counts as at least that many', () => {
		const conversations = [direct('dm-1', 0, { unread: { status: 'at_least', value: 20 } })];
		expect(messengerUnreadTotal(conversations, [], nobodyMuted).addressedToMe).toBe(20);
	});
});
