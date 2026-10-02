import { describe, expect, test } from 'bun:test';
import {
	jumpToLatestLabel,
	unseenMessageCount
} from '../../src/lib/components/channel/channel-unseen-messages';

type TestMessage = { id: string; sentAt: string; senderID: string; threadRootId?: string };

const seenThrough = '2026-10-02T09:00:00.000Z';
const isMine = (message: TestMessage): boolean => message.senderID === 'me';

const text = {
	jumpToLatest: 'Jump to latest',
	unseenMessageOne: '1 new message',
	unseenMessageMany: '{count} new messages'
};

describe('unseenMessageCount', () => {
	test('counts only messages from others that arrived after the reader scrolled away', () => {
		const messages: TestMessage[] = [
			{ id: 'old', sentAt: '2026-10-02T08:59:00.000Z', senderID: 'other' },
			{ id: 'seen', sentAt: seenThrough, senderID: 'other' },
			{ id: 'mine', sentAt: '2026-10-02T09:01:00.000Z', senderID: 'me' },
			{ id: 'reply', sentAt: '2026-10-02T09:02:00.000Z', senderID: 'other', threadRootId: 'old' },
			{ id: 'new-1', sentAt: '2026-10-02T09:03:00.000Z', senderID: 'other' },
			{ id: 'new-2', sentAt: '2026-10-02T09:04:00.000Z', senderID: 'agent' }
		];
		expect(unseenMessageCount(messages, seenThrough, isMine)).toBe(2);
	});

	test('counts nothing before the conversation has a settled message', () => {
		const messages: TestMessage[] = [{ id: 'new', sentAt: seenThrough, senderID: 'other' }];
		expect(unseenMessageCount(messages, '', isMine)).toBe(0);
	});
});

describe('jumpToLatestLabel', () => {
	test('names the count once something new has arrived', () => {
		expect(jumpToLatestLabel(0, text)).toBe('Jump to latest');
		expect(jumpToLatestLabel(1, text)).toBe('1 new message');
		expect(jumpToLatestLabel(3, text)).toBe('3 new messages');
	});
});
