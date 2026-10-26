import { describe, expect, test } from 'bun:test';
import type { InboxEntry } from '../../../src/lib/messenger/channel-over-bridge';
import { mergeInbox } from '../../../src/lib/messenger/inbox-cache';

function entry(id: string, sentAt: string, text = id, isUnread = true): InboxEntry {
	return {
		id,
		conversationID: 'dm-1',
		reason: 'direct',
		isUnread,
		sender: { id: 'person-a', name: '이샘플' },
		text,
		sentAt
	};
}

describe('an inbox brought up to date', () => {
	const held = [entry('second', '2026-10-12T05:02:00.000Z'), entry('first', '2026-10-12T05:01:00.000Z')];

	test('puts what arrived above what it held, and takes the held wording and read state from the answer', () => {
		const merged = mergeInbox(held, {
			entries: [entry('third', '2026-10-12T05:03:00.000Z')],
			held: [
				{ id: 'second', text: 'second, reworded', isUnread: false },
				{ id: 'first', text: 'first', isUnread: true }
			]
		});

		expect(merged.map((each) => [each.id, each.text, each.isUnread])).toEqual([
			['third', 'third', true],
			['second', 'second, reworded', false],
			['first', 'first', true]
		]);
	});

	test('drops a held entry the answer no longer holds, as when it was taken back', () => {
		const merged = mergeInbox(held, { entries: [], held: [{ id: 'first', text: 'first', isUnread: true }] });

		expect(merged.map((each) => each.id)).toEqual(['first']);
	});

	test('shows a message once when it is both arrived and held', () => {
		const merged = mergeInbox(held, {
			entries: [entry('second', '2026-10-12T05:02:00.000Z')],
			held: [
				{ id: 'second', text: 'second', isUnread: true },
				{ id: 'first', text: 'first', isUnread: true }
			]
		});

		expect(merged.map((each) => each.id)).toEqual(['second', 'first']);
	});

	test('takes an answer without held entries as the whole inbox', () => {
		const merged = mergeInbox(held, { entries: [entry('third', '2026-10-12T05:03:00.000Z')] });

		expect(merged.map((each) => each.id)).toEqual(['third']);
	});
});
