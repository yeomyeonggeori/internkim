import { describe, expect, test } from 'bun:test';
import type { InboxEntry } from '../../../src/lib/messenger/channel-over-bridge';
import { appendOlderInbox } from '../../../src/lib/messenger/inbox-cache';

function entry(id: string, sentAt: string): InboxEntry {
	return {
		id,
		conversationID: 'channel-1',
		reason: 'mention',
		isUnread: true,
		sender: { id: 'person-a', name: '이샘플' },
		text: id,
		sentAt
	};
}

describe('an inbox scrolled to its older entries', () => {
	test('puts the older page below what it shows and shows the entry on the page boundary once', () => {
		const shown = [entry('second', '2026-10-12T05:02:00.000Z'), entry('first', '2026-10-12T05:01:00.000Z')];

		const appended = appendOlderInbox(shown, [entry('first', '2026-10-12T05:01:00.000Z'), entry('zeroth', '2026-10-12T05:00:00.000Z')]);

		expect(appended.map((each) => each.id)).toEqual(['second', 'first', 'zeroth']);
	});
});
