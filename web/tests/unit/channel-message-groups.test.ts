import { describe, expect, test } from 'bun:test';
import { groupConsecutiveMessages } from '../../src/lib/components/channel/channel-message-groups';
import type {
	ChannelMessage,
	ChannelParticipant,
	ThreadSummary
} from '../../src/lib/components/channel/channel-api';

function makeParticipant(id: string, name: string): ChannelParticipant {
	return { id, name, email: `${id}@example.com` };
}

function makeMessage(
	id: string,
	senderID: string,
	senderName: string,
	text: string,
	thread?: ThreadSummary
): ChannelMessage {
	return {
		id,
		sender: makeParticipant(senderID, senderName),
		text,
		sentAt: '2026-09-20T09:00:00.000Z',
		thread
	};
}

describe('groupConsecutiveMessages', () => {
	test('consecutive messages from the same sender collapse into one group', () => {
		const messages = [
			makeMessage('m1', 'sender1', '이샘플', 'hello'),
			makeMessage('m2', 'sender1', '이샘플', 'how are you'),
			makeMessage('m3', 'sender1', '이샘플', 'goodbye')
		];

		const groups = groupConsecutiveMessages(messages);

		expect(groups).toHaveLength(1);
		expect(groups[0]).toEqual({ id: 'm1', senderID: 'sender1', items: messages });
	});

	test('alternating senders produce one group each', () => {
		const messages = [
			makeMessage('m1', 'sender1', '이샘플', 'hello'),
			makeMessage('m2', 'sender2', '박예시', 'hi there'),
			makeMessage('m3', 'sender1', '이샘플', 'how are you'),
			makeMessage('m4', 'sender2', '박예시', 'doing great')
		];

		const groups = groupConsecutiveMessages(messages);

		expect(groups.map((group) => group.senderID)).toEqual([
			'sender1',
			'sender2',
			'sender1',
			'sender2'
		]);
		expect(groups.map((group) => group.items.length)).toEqual([1, 1, 1, 1]);
	});

	test('a single message produces a single group of one', () => {
		const messages = [makeMessage('m1', 'sender1', '이샘플', 'hello')];

		const groups = groupConsecutiveMessages(messages);

		expect(groups).toHaveLength(1);
		expect(groups[0]).toEqual({ id: 'm1', senderID: 'sender1', items: messages });
	});

	test('an empty list produces an empty array', () => {
		expect(groupConsecutiveMessages([])).toEqual([]);
	});

	test('a message that opens a thread ends its group', () => {
		const thread: ThreadSummary = {
			replyCount: 2,
			lastReplyAt: '2026-09-20T09:05:00.000Z',
			participants: [makeParticipant('sender1', '이샘플')]
		};
		const messages = [
			makeMessage('m1', 'sender1', '이샘플', 'message with thread', thread),
			makeMessage('m2', 'sender1', '이샘플', 'next message from same sender'),
			makeMessage('m3', 'sender2', '박예시', 'other sender'),
			makeMessage('m4', 'sender1', '이샘플', 'another message from first sender')
		];

		const groups = groupConsecutiveMessages(messages);

		expect(groups.map((group) => group.id)).toEqual(['m1', 'm2', 'm3', 'm4']);
		expect(groups.map((group) => group.items.length)).toEqual([1, 1, 1, 1]);
	});
});
