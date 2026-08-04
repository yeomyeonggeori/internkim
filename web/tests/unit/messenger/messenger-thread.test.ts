import { describe, expect, test } from 'bun:test';
import { channelLabel, threadsOf } from '../../../src/routes/messenger/messenger-thread';
import type { MessengerChannel, MessengerPost } from '../../../src/lib/messenger/messenger-api';
import type { MessengerDirectory } from '../../../src/lib/messenger/messenger-directory';

function post(id: string, parentID?: string): MessengerPost {
	return {
		id,
		channelID: 'c1',
		parentID,
		author: { externalID: 'U1' },
		body: id,
		postedAt: '2026-08-04T00:00:00Z',
		reactions: []
	};
}

function directoryOf(): MessengerDirectory {
	return {
		nameOfMember: new Map([['m1', '이샘플']]),
		nameOfExternal: new Map([['U9', '김철수']]),
		memberOfExternal: new Map([['U7', 'm1']])
	};
}

function channelOf(fields: Partial<MessengerChannel>): MessengerChannel {
	return { id: 'c1', platform: 'mattermost', name: '이름없음', isDirect: false, position: 0, participants: [], ...fields };
}

describe('threadsOf', () => {
	test('a reply hangs under the post it answers', () => {
		const threads = threadsOf([post('a'), post('b', 'a')]);
		expect(threads).toHaveLength(1);
		expect(threads[0].replies.map((reply) => reply.id)).toEqual(['b']);
	});

	test('posts keep the order they came in', () => {
		const threads = threadsOf([post('a'), post('b'), post('c', 'a')]);
		expect(threads.map((thread) => thread.post.id)).toEqual(['a', 'b']);
	});

	test('a reply whose post is out of view still shows', () => {
		const threads = threadsOf([post('b', 'missing')]);
		expect(threads.map((thread) => thread.post.id)).toEqual(['b']);
	});

	test('replies to the same post stay together in order', () => {
		const threads = threadsOf([post('a'), post('b', 'a'), post('c', 'a')]);
		expect(threads[0].replies.map((reply) => reply.id)).toEqual(['b', 'c']);
	});
});

describe('channelLabel', () => {
	test('an ordinary channel keeps its own name', () => {
		expect(channelLabel(channelOf({ name: '광장' }), directoryOf())).toBe('광장');
	});

	test('a direct channel is named after who is in it', () => {
		const channel = channelOf({ isDirect: true, name: '', participants: [{ externalID: 'U9' }, { externalID: 'U7' }] });
		expect(channelLabel(channel, directoryOf())).toBe('김철수, 이샘플');
	});

	test('a direct channel with nobody recognised falls back to its own name', () => {
		const channel = channelOf({ isDirect: true, name: 'group chat', participants: [{ externalID: 'UNSEEN' }] });
		expect(channelLabel(channel, directoryOf())).toBe('group chat');
	});

	test('before the directory loads, the channel still has a name', () => {
		const channel = channelOf({ isDirect: true, name: 'dm', participants: [{ externalID: 'U9' }] });
		expect(channelLabel(channel, null)).toBe('dm');
	});
});
