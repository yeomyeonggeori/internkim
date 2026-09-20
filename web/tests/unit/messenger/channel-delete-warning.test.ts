import { describe, expect, test } from 'bun:test';
import { deleteWarningFor } from '../../../src/lib/components/channel/channel-delete-warning';
import type { ChannelMessage } from '../../../src/lib/components/channel/channel-api';

const text = {
	deleteMessageDescription: 'just this one',
	deleteThreadDescription: 'the replies go too'
};

const writer = { id: 'p1', name: '이샘플' };

function message(replyCount?: number): ChannelMessage {
	const written: ChannelMessage = {
		id: 'm1',
		sender: writer,
		text: '안녕하세요',
		sentAt: '2026-09-20T00:00:00Z'
	};
	if (replyCount === undefined) return written;
	return {
		...written,
		thread: { replyCount, lastReplyAt: '2026-09-20T00:01:00Z', participants: [writer] }
	};
}

describe('the warning shown before a message is deleted', () => {
	test('says the replies go too when the message has some', () => {
		expect(deleteWarningFor(message(3), text)).toBe('the replies go too');
	});

	test('says it is only this message when nobody answered', () => {
		expect(deleteWarningFor(message(0), text)).toBe('just this one');
	});

	test('says it is only this message when the message carries no thread at all', () => {
		expect(deleteWarningFor(message(), text)).toBe('just this one');
	});
});
