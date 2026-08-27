import { describe, expect, test } from 'bun:test';
import { threadsOf } from '../../../src/routes/messenger/messenger-thread';
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
		reactions: [],
		attachments: []
	};
}

function directoryOf(): MessengerDirectory {
	return {
		nameOfMember: new Map([['m1', '이샘플']]),
		nameOfExternal: new Map([['U9', '김철수']]),
		externalsOfMember: new Map(),
		memberOfExternal: new Map([['U7', 'm1']]),
		memberOfEmail: new Map()
	};
}

function channelOf(fields: Partial<MessengerChannel>): MessengerChannel {
	return { id: 'c1', platform: 'mattermost', name: '이름없음', isDirect: false, isPrivate: false, position: 0, participants: [], ...fields };
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

