import { afterEach, describe, expect, test } from 'bun:test';
import type { ChannelMessage, ChannelParticipant } from '../../../src/lib/components/channel/channel-api';
import { clearChannelMessageCache, getCachedMessages, setCachedMessages } from '../../../src/lib/components/channel/channel-message-cache';
import { threadRepliesByRoot, timelineMessages } from '../../../src/lib/components/channel/channel-threads';

const rootAuthor: ChannelParticipant = { id: 'sample-root-author', name: '이샘플' };
const replyAuthor: ChannelParticipant = { id: 'sample-reply-author', name: '박예시' };
const anotherAuthor: ChannelParticipant = { id: 'sample-another-author', name: '최견본' };

function message(id: string, sender: ChannelParticipant, minute: number, threadRootId?: string): ChannelMessage {
	return { id, sender, threadRootId, text: id, sentAt: `2026-10-06T03:${String(minute).padStart(2, '0')}:00Z` };
}

function timeline(messages: ChannelMessage[]): ChannelMessage[] {
	return timelineMessages(messages, threadRepliesByRoot(messages));
}

afterEach(clearChannelMessageCache);

describe('thread summaries describe the loaded replies', () => {
	test('a root without replies has no reply chip', () => {
		const root = message('root', rootAuthor, 0);
		expect(timeline([root])).toEqual([root]);
		expect(timeline([root])[0].thread).toBeUndefined();
	});

	test('one reply shows its author without adding the root author', () => {
		const root = message('root', rootAuthor, 0);
		const reply = message('reply', replyAuthor, 1, root.id);
		expect(timeline([root, reply])[0].thread).toEqual({
			replyCount: 1, lastReplyAt: reply.sentAt, participants: [replyAuthor]
		});
	});

	test('the root author appears when they actually reply', () => {
		const root = message('root', rootAuthor, 0);
		const reply = message('self-reply', rootAuthor, 1, root.id);
		expect(timeline([root, reply])[0].thread?.participants).toEqual([rootAuthor]);
	});

	test('repeated authors contribute one avatar while every reply is counted', () => {
		const root = message('root', rootAuthor, 0);
		const replies = [
			message('reply-1', replyAuthor, 1, root.id),
			message('reply-2', rootAuthor, 2, root.id),
			message('reply-3', replyAuthor, 3, root.id),
			message('reply-4', anotherAuthor, 4, root.id)
		];
		expect(timeline([root, ...replies])[0].thread).toEqual({
			replyCount: 4, lastReplyAt: replies[3].sentAt, participants: [replyAuthor, rootAuthor, anotherAuthor]
		});
	});

	test('reply order and last reply time follow timestamps, not page arrival order', () => {
		const root = message('root', rootAuthor, 0);
		const first = message('reply-1', replyAuthor, 1, root.id);
		const last = message('reply-2', anotherAuthor, 2, root.id);
		expect(timeline([root, last, first])[0].thread).toEqual({
			replyCount: 2, lastReplyAt: last.sentAt, participants: [replyAuthor, anotherAuthor]
		});
	});

	test('removing replies removes the final avatar for their author and the last reply removes the chip', () => {
		const root = message('root', rootAuthor, 0);
		const first = message('reply-1', replyAuthor, 1, root.id);
		const second = message('reply-2', replyAuthor, 2, root.id);
		const last = message('reply-3', anotherAuthor, 3, root.id);
		const messages = [root, first, second, last];
		expect(timeline(messages.filter(entry => entry.id !== first.id))[0].thread?.participants).toEqual([replyAuthor, anotherAuthor]);
		const remaining = messages.filter(entry => entry.sender.id !== replyAuthor.id);
		expect(timeline(remaining)[0].thread).toEqual({ replyCount: 1, lastReplyAt: last.sentAt, participants: [anotherAuthor] });
		expect(timeline([root])[0].thread).toBeUndefined();
	});

	test('a later history page supplies the root without counting its author as a reply', () => {
		const root = message('root', rootAuthor, 0);
		const first = message('reply-1', replyAuthor, 1, root.id);
		const last = message('reply-2', anotherAuthor, 2, root.id);
		expect(timeline([last])).toEqual([last]);
		const combinedPages = [root, first, last];
		expect(timeline(combinedPages)).toHaveLength(1);
		expect(timeline(combinedPages)[0].thread?.participants).toEqual([replyAuthor, anotherAuthor]);
	});

	test('cached raw messages stay unmodified and rebuild the avatars after fresh data arrives', () => {
		const root = message('root', rootAuthor, 0);
		const reply = message('reply', replyAuthor, 1, root.id);
		setCachedMessages('sample-channel', [root, reply]);
		expect(timeline(getCachedMessages('sample-channel') ?? [])[0].thread?.participants).toEqual([replyAuthor]);
		expect(getCachedMessages('sample-channel')?.[0].thread).toBeUndefined();
		const selfReply = message('self-reply', rootAuthor, 2, root.id);
		setCachedMessages('sample-channel', [root, selfReply]);
		expect(timeline(getCachedMessages('sample-channel') ?? [])[0].thread?.participants).toEqual([rootAuthor]);
		setCachedMessages('sample-channel', [root]);
		expect(timeline(getCachedMessages('sample-channel') ?? [])[0].thread).toBeUndefined();
	});

	test('separate threads never share participants', () => {
		const firstRoot = message('root-1', rootAuthor, 0);
		const secondRoot = message('root-2', anotherAuthor, 1);
		const summaries = timeline([
			firstRoot, secondRoot,
			message('reply-1', replyAuthor, 2, firstRoot.id),
			message('reply-2', rootAuthor, 3, secondRoot.id)
		]);
		expect(summaries.map(root => root.thread?.participants)).toEqual([[replyAuthor], [rootAuthor]]);
	});
});
